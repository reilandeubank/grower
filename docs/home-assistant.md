# Home Assistant

grower can tell Home Assistant what needs care, so plant care can show up on a dashboard, in an automation or as a spoken announcement, not only in the daily web push.
There are two ways, and you can use either or both; neither needs a custom component:

- **REST sensor:** one sensor with today's summary, read with an API token (below).
- **[MQTT with discovery](#mqtt-with-discovery):** a Home Assistant device per plant, with sensors and a button per care task that marks it done.
  Needs an MQTT broker, such as the Mosquitto add-on.

## REST sensor

### 1. Create an API token

In grower, open **Settings → Integrations → Create API token**, keep or change the name, and copy the token.
grower shows it once and stores only a fingerprint of it.

Tokens are per user and read-only: they can read the summary below and nothing else, not even the rest of the API.
The summary uses the token owner's time zone and hemisphere.
Revoke a token in the same place; it stops working immediately.

### 2. Keep the token in `secrets.yaml`

```yaml
# secrets.yaml
grower_token: "Bearer grower_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
```

The value includes the word `Bearer` and a space, because it goes straight into the `Authorization` header.

### 3. Add the sensor

```yaml
# configuration.yaml
rest:
  - resource: https://grower.example.com/api/v1/integrations/home-assistant
    headers:
      Authorization: !secret grower_token
    scan_interval: 300
    sensor:
      - name: Plants needing care
        unique_id: grower_needs_care
        icon: mdi:sprout
        unit_of_measurement: tasks
        value_template: "{{ value_json.needs_care }}"
        json_attributes:
          - date
          - overdue
          - due
          - upcoming
          - plants
          - title
          - message
          - items
```

Replace the address with your `GROWER_BASE_URL`.
If Home Assistant reaches grower on your local network, you can use the local address instead (`http://grower-host:8080/...`); the token works on either.
After editing, restart Home Assistant or reload the RESTful integration.

A five-minute `scan_interval` is plenty: the summary only changes when you log care or a new day starts.
If grower is unreachable, the sensor becomes `unavailable` until it's back.

### The response

`GET /api/v1/integrations/home-assistant` with `Authorization: Bearer <token>` returns:

```json
{
  "date": "2026-10-28",
  "needs_care": 3,
  "overdue": 1,
  "due": 2,
  "upcoming": 4,
  "plants": 3,
  "title": "3 plants need care",
  "message": "Water Pothos (Office), Mist Fern and Check soil for Monstera",
  "items": [
    {
      "plant_id": "01J...",
      "plant": "Pothos",
      "location": "Office",
      "task_id": "01J...",
      "task": "water",
      "label": "Water Pothos (Office)",
      "days_overdue": 1
    }
  ]
}
```

- `needs_care` is overdue plus due today, which makes a good sensor state; `upcoming` counts the next 7 days.
- `title` and `message` are the daily reminder's own words, and are empty when nothing is due.
- `items` lists overdue tasks (most overdue first), then today's.
  `label` is how the reminder names each one, with the room added when two plants share a name.

The endpoint also accepts the session cookie, so you can open it in a signed-in browser to check what Home Assistant will see.

### Example automations

Announce what needs care when you get home:

```yaml
automation:
  - alias: Plant care when arriving home
    triggers:
      - trigger: state
        entity_id: person.you
        to: home
    conditions:
      - condition: numeric_state
        entity_id: sensor.plants_needing_care
        above: 0
    actions:
      - action: notify.mobile_app_your_phone
        data:
          title: "{{ state_attr('sensor.plants_needing_care', 'title') }}"
          message: "{{ state_attr('sensor.plants_needing_care', 'message') }}"
```

Show the list on a dashboard with a Markdown card:

```yaml
type: markdown
title: Plants
content: >-
  {% set items = state_attr('sensor.plants_needing_care', 'items') or [] %}
  {% for it in items %}
  - {{ it.label }}{% if it.days_overdue > 0 %} ({{ it.days_overdue }} days late){% endif %}
  {% else %}
  Nothing needs care today.
  {% endfor %}
```

To mark tasks done from Home Assistant, use MQTT.

## MQTT with discovery

With an MQTT broker, grower shows up in Home Assistant by itself (MQTT discovery): no YAML, and no API token.
It publishes a **grower** device with the same summary as the REST sensor, and one device per plant that has care tasks, placed in the area named like its room.

Each plant device has:

| Entity | What it shows |
| --- | --- |
| Needs care (problem) | On when any of the plant's tasks is overdue or due today. |
| Next care (date) | The plant's soonest due date. |
| Days overdue | How late its latest task is; 0 when nothing is late. |
| Last watered (date) | Only for plants with a watering task. |
| Soil moisture | The latest meter reading (1–10), only for plants with a soil-check watering. |
| *Task* due (date) | Each task's next due date: "Water due", "Fertilize due", "Prune due", "Next soil check". |
| *Task* done (button) | Marks that task done today, as tapping it in grower does: "Water done", or "Watered" for a soil check. |

The grower device has **Needs care**, **Overdue** and **Upcoming** counts; **Needs care** carries the whole summary (`title`, `message`, `items`) as attributes.

### Set it up

1. In Home Assistant, install a broker if you don't have one (**Settings → Add-ons → Mosquitto broker**), and set up the **MQTT** integration.
   Create a Home Assistant user for grower to log in to the broker with.
2. Add these to grower's `environment:` and restart:

   ```yaml
   GROWER_MQTT_URL: mqtt://homeassistant.local:1883
   GROWER_MQTT_USERNAME: grower
   GROWER_MQTT_PASSWORD: the-password
   ```

3. The startup log says `mqtt: connected`, and the devices appear under **Settings → Devices & services → MQTT**.

`GROWER_MQTT_URL` takes `mqtt://` (port 1883 by default), `mqtts://` (TLS, 8883), `ws://` or `wss://`.
Credentials can also go in the URL (`mqtt://user:password@host`), percent-encoded; the separate variables win.
If the broker can't be reached, grower runs normally, keeps trying, and logs a warning after 30 seconds.

| Variable | Default | |
| --- | --- | --- |
| `GROWER_MQTT_URL` | (off) | The broker. MQTT is off while it's unset. |
| `GROWER_MQTT_USERNAME`, `GROWER_MQTT_PASSWORD` | none | Broker credentials. The password is never logged. |
| `GROWER_MQTT_TOPIC` | `grower` | Prefix for grower's topics, and for its device and entity IDs. Give each grower its own if several share a broker. |
| `GROWER_MQTT_DISCOVERY_PREFIX` | `homeassistant` | Home Assistant's discovery prefix, if you changed it. |

### How it behaves

- Changes in grower reach Home Assistant within a second or two, and the dates roll over within a minute of midnight.
- Plants are household-wide, so the dates use the time zone and hemisphere of the first account (the one made at setup).
- A button press logs the task done today. A second press the same day does nothing, so a double tap or an automation firing twice doesn't log twice.
  Undo it in grower if you pressed by mistake.
- Archiving or deleting a plant removes its device; deleting a task removes its entities. This also works for changes made while grower was stopped.
- While grower is stopped, its entities show as unavailable.

Anyone who can publish to the broker can press the buttons, so give the broker accounts (Mosquitto has them by default) and keep it off the internet.

### Topics

grower's messages are retained, so Home Assistant has them after a restart:

- `grower/status`: `online` or `offline` (the broker sets `offline` if grower disappears).
- `grower/summary`: the REST sensor's JSON.
- `grower/plant/<plant id>/state`: `needs_care`, `next_due`, `days_overdue`, `last_watered`, `moisture`, and `tasks` by task ID with each one's `next_due`, `days_overdue` and `last_done`.
- `grower/task/<task id>/done`: publish anything (not retained) to mark the task done.
- `homeassistant/device/grower…/config`: discovery.

An example automation that uses a plant's entity:

```yaml
automation:
  - alias: Remind me about thirsty plants in the evening
    triggers:
      - trigger: time
        at: "18:00:00"
    conditions:
      - condition: state
        entity_id: binary_sensor.monstera_needs_care
        state: "on"
    actions:
      - action: notify.mobile_app_your_phone
        data:
          message: "The Monstera needs care."
```
