package push

import (
	"fmt"
	"strings"

	"github.com/reilandeubank/grower/internal/care"
)

// listed is how many tasks the reminder names before "and N more".
const listed = 3

// TestMessage is the notification Settings sends to check a device.
var TestMessage = Message{Title: "grower", Body: "Notifications are working.", URL: "/settings", Tag: "test"}

// DigestMessage words the daily reminder: "2 plants need care" / "Water Monstera, Mist
// Fern and 1 more", overdue tasks first. ok is false when nothing is due.
func DigestMessage(a care.Agenda) (m Message, ok bool) {
	items := append(append([]care.Item{}, a.Overdue...), a.Due...)
	if len(items) == 0 {
		return Message{}, false
	}
	plants := map[string]bool{}
	names := make([]string, 0, listed+1)
	for i, it := range items {
		plants[it.Plant.ID] = true
		if i < listed {
			names = append(names, taskPhrase(it))
		}
	}
	if more := len(items) - listed; more > 0 {
		names = append(names, fmt.Sprintf("%d more", more))
	}
	title := "1 plant needs care"
	if n := len(plants); n != 1 {
		title = fmt.Sprintf("%d plants need care", n)
	}
	return Message{Title: title, Body: joinAnd(names), URL: "/", Tag: "digest"}, true
}

var verbs = map[string]string{
	"water": "Water", "fertilize": "Fertilize", "mist": "Mist", "repot": "Repot", "rotate": "Rotate", "check_in": "Check in on",
}

// taskPhrase is "Water Monstera", "Check in on Monstera", "Check soil for Monstera" for
// a soil-check task, or "Prune for Monstera" for a custom task.
func taskPhrase(it care.Item) string {
	if it.Task.MoistureCheck == 1 {
		return "Check soil for " + it.Plant.Name
	}
	if verb, ok := verbs[it.Task.Kind]; ok {
		return verb + " " + it.Plant.Name
	}
	return it.Task.Label + " for " + it.Plant.Name
}

// joinAnd is "A", "A and B", or "A, B and C".
func joinAnd(s []string) string {
	if len(s) < 2 {
		return strings.Join(s, "")
	}
	return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
}
