# Home Assistant

grower can tell Home Assistant what needs care today, so plant care can show up on a dashboard, in an automation or as a spoken announcement, not only in the daily web push.
Home Assistant reads it with its built-in [RESTful sensor](https://www.home-assistant.io/integrations/sensor.rest/); no custom component is needed.

## 1. Create an API token

In grower, open **Settings → Integrations → Create API token**, keep or change the name, and copy the token.
grower shows it once and stores only a fingerprint of it.

Tokens are per user and read-only: they can read the summary below and nothing else, not even the rest of the API.
The summary uses the token owner's time zone and hemisphere.
Revoke a token in the same place; it stops working immediately.

## 2. Keep the token in `secrets.yaml`

```yaml
# secrets.yaml
grower_token: "Bearer grower_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
```

The value includes the word `Bearer` and a space, because it goes straight into the `Authorization` header.

## 3. Add the sensor

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

## The response

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

## Example automations

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

Marking tasks done from Home Assistant isn't supported yet; tap them off in grower.
