package mqtt

import (
	"context"
	"errors"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/reilandeubank/grower/internal/config"
)

const (
	// syncEvery catches what pokes miss: a new day, or a change made with the CLI.
	syncEvery      = time.Minute
	publishTimeout = 10 * time.Second
	// warnAfter is how long to wait for a first connection before saying it isn't coming.
	warnAfter = 30 * time.Second
	// inboxSize bounds messages waiting for the loop. Beyond it they are dropped (and
	// logged); the broker replays retained discovery on the next reconnect.
	inboxSize = 512
)

type message struct {
	topic    string
	payload  []byte
	retained bool
}

// pahoClient publishes retained messages at QoS 1 and waits for the broker to take them.
type pahoClient struct{ c paho.Client }

func (pc pahoClient) Publish(topic string, payload []byte) error {
	tok := pc.c.Publish(topic, 1, true, payload)
	if !tok.WaitTimeout(publishTimeout) {
		return errors.New("timed out")
	}
	return tok.Error()
}

// Run connects to the broker in cfg and keeps it in step until ctx ends, reconnecting
// as needed. It sets the status topic to "online", and the broker sets it to "offline"
// (the last will) if grower goes away without saying so.
func (p *Publisher) Run(ctx context.Context, cfg *config.MQTT) {
	inbox := make(chan message, inboxSize)
	connected := make(chan struct{}, 1)
	receive := func(_ paho.Client, m paho.Message) {
		select {
		case inbox <- message{topic: m.Topic(), payload: m.Payload(), retained: m.Retained()}:
		default:
			p.Log.Warn("mqtt: dropped a message; too many waiting", "topic", m.Topic())
		}
	}

	opts := paho.NewClientOptions().
		AddBroker(cfg.Broker.String()).
		SetClientID("grower-"+p.Topics.ID).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetCleanSession(true).
		SetOrderMatters(false).
		SetKeepAlive(60*time.Second).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(10*time.Second).
		SetMaxReconnectInterval(5*time.Minute).
		SetWill(p.Topics.Status(), "offline", 1, true).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			p.Log.Warn("mqtt: lost the broker connection; reconnecting", "err", err)
		}).
		SetOnConnectHandler(func(c paho.Client) {
			subs := map[string]byte{p.Topics.TaskDone("+"): 1, p.Topics.HAStatus(): 1, p.Topics.DeviceConfigs(): 1}
			if tok := c.SubscribeMultiple(subs, receive); !tok.WaitTimeout(publishTimeout) || tok.Error() != nil {
				p.Log.Error("mqtt: subscribe failed", "err", tok.Error())
			}
			if err := (pahoClient{c}).Publish(p.Topics.Status(), []byte("online")); err != nil {
				p.Log.Error("mqtt: publish status failed", "err", err)
			}
			select {
			case connected <- struct{}{}:
			default:
			}
		})
	client := paho.NewClient(opts)
	p.Log.Info("mqtt: connecting", "broker", cfg.Broker.String(), "topic", p.Topics.Base, "discovery_prefix", p.Topics.Discovery)
	client.Connect() // Retries in the background until it connects.
	pc := pahoClient{client}

	sync := func() {
		if !client.IsConnectionOpen() {
			return
		}
		if err := p.Sync(ctx, pc); err != nil && ctx.Err() == nil {
			p.Log.Error("mqtt: sync failed", "err", err)
		}
	}
	ticker := time.NewTicker(syncEvery)
	defer ticker.Stop()
	unreachable := time.NewTimer(warnAfter)
	defer unreachable.Stop()
	for {
		select {
		case <-unreachable.C:
			if !client.IsConnectionOpen() {
				p.Log.Warn("mqtt: can't reach the broker yet; still trying. Check GROWER_MQTT_URL and the broker's credentials.", "broker", cfg.Broker.String())
			}
		case <-ctx.Done():
			if client.IsConnectionOpen() {
				_ = pc.Publish(p.Topics.Status(), []byte("offline"))
			}
			client.Disconnect(250)
			return
		case <-connected:
			p.Log.Info("mqtt: connected", "broker", cfg.Broker.String())
			p.forget()
			sync()
		case <-p.pokes:
			sync()
		case <-ticker.C:
			sync()
		case m := <-inbox:
			switch {
			case m.topic == p.Topics.HAStatus():
				if string(m.payload) == "online" {
					p.forget()
					sync()
				}
			case p.Discovered(m.topic, m.payload):
				sync()
			default:
				changed, err := p.Pressed(ctx, m.topic, m.retained)
				if err != nil {
					p.Log.Error("mqtt: button press failed", "topic", m.topic, "err", err)
				}
				if changed {
					sync()
				}
			}
		}
	}
}
