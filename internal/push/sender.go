package push

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/SherClockHolmes/webpush-go"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

const (
	// ttl is how long the push service holds a message for an offline device; a
	// reminder older than half a day is no longer worth showing.
	ttl = 12 * time.Hour
	// sendTimeout bounds one delivery to one push service.
	sendTimeout = 30 * time.Second
)

// Message is the notification payload the service worker shows (web/src/sw.ts).
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	// URL is the app path a tap opens.
	URL string `json:"url"`
	// Tag replaces an earlier notification with the same tag on the device.
	Tag string `json:"tag"`
}

// Sender delivers messages to a user's subscribed devices.
type Sender struct {
	DB   *db.DB
	Keys Keys
	// Subject identifies the sender to push services: GROWER_BASE_URL.
	Subject string
	// Client makes the HTTP requests; nil uses a client with a timeout.
	Client webpush.HTTPClient
	Now    func() time.Time
	Log    *slog.Logger
}

// ErrUndelivered means no device received the message, and at least one may on a retry.
var ErrUndelivered = errors.New("push not delivered")

// Send delivers m to every device of the user. Devices the push service reports gone
// (404, 410) are removed. It fails only when no device got the message and one failed
// for another reason, so retrying never repeats a delivery that worked.
func (s *Sender) Send(ctx context.Context, userID string, m Message) error {
	subs, err := s.DB.Reader().ListUserPushSubscriptions(ctx, userID)
	if err != nil {
		return fmt.Errorf("list push subscriptions: %w", err)
	}
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var delivered int
	var errs []error
	for _, sub := range subs {
		if err := s.deliver(ctx, sub, payload, m.Tag); err != nil {
			errs = append(errs, err)
			continue
		}
		delivered++
	}
	if delivered == 0 && len(errs) > 0 {
		return fmt.Errorf("%w: %w", ErrUndelivered, errors.Join(errs...))
	}
	for _, err := range errs { // some device got it: log the rest, don't retry
		s.Log.WarnContext(ctx, "push to a device failed", "err", err)
	}
	return nil
}

// errGone marks a subscription the push service no longer knows; it is not a failure.
var errGone = errors.New("subscription gone")

// deliver sends one message and records the outcome on the subscription. A gone
// subscription is deleted and counts as handled.
func (s *Sender) deliver(ctx context.Context, sub dbgen.PushSubscription, payload []byte, topic string) error {
	err := s.post(ctx, sub, payload, topic)
	q := s.DB.Writer()
	switch {
	case errors.Is(err, errGone):
		s.Log.InfoContext(ctx, "push subscription gone; removed", "subscription", sub.ID)
		if derr := q.DeletePushSubscription(ctx, sub.ID); derr != nil {
			return fmt.Errorf("delete push subscription %s: %w", sub.ID, derr)
		}
		return nil
	case err != nil:
		if merr := q.MarkPushFailed(ctx, sub.ID); merr != nil {
			err = errors.Join(err, merr)
		}
		return fmt.Errorf("push to subscription %s: %w", sub.ID, err)
	}
	return q.MarkPushDelivered(ctx, dbgen.MarkPushDeliveredParams{LastOkAt: sql.NullString{String: db.FormatTime(s.Now()), Valid: true}, ID: sub.ID})
}

func (s *Sender) post(ctx context.Context, sub dbgen.PushSubscription, payload []byte, topic string) error {
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: sendTimeout}
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient:      client,
		Subscriber:      s.Subject,
		Topic:           topic,
		TTL:             int(ttl.Seconds()),
		Urgency:         webpush.UrgencyNormal,
		VAPIDPublicKey:  s.Keys.Public,
		VAPIDPrivateKey: s.Keys.Private,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return errGone
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("push service answered %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	return nil
}
