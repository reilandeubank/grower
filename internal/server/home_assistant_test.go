package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
)

const haPath = "/api/v1/integrations/home-assistant"

// newToken creates an API token as the cookie's user and returns it with its secret.
func (e *env) newToken(c *http.Cookie, label string) apigen.ApiTokenCreated {
	e.t.Helper()
	return expectJSON[apigen.ApiTokenCreated](e.t, e.call(c, http.MethodPost, "/api/v1/api-tokens", `{"label":"`+label+`"}`), http.StatusCreated)
}

func (e *env) tokens(c *http.Cookie) []apigen.ApiToken {
	e.t.Helper()
	return expectJSON[apigen.ApiTokenList](e.t, e.call(c, http.MethodGet, "/api/v1/api-tokens", ""), http.StatusOK).Items
}

// withBearer calls path with only an Authorization header, as Home Assistant does.
func (e *env) withBearer(secret, method, path string) *http.Response {
	e.t.Helper()
	return e.do(request{method: method, path: path, headers: map[string]string{"Authorization": "Bearer " + secret}})
}

func TestAPITokensLifecycle(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()

	if got := e.tokens(c); len(got) != 0 {
		t.Fatalf("tokens before any = %+v", got)
	}
	created := e.newToken(c, "  Home Assistant ")
	if !strings.HasPrefix(created.Secret, "grower_") || len(created.Secret) < 40 {
		t.Errorf("secret = %q, want a long grower_ token", created.Secret)
	}
	if created.Token.Label != "Home Assistant" || created.Token.Scope != "read" || created.Token.LastUsedAt != nil {
		t.Errorf("token = %+v", created.Token)
	}

	// The secret is never listed, and only its hash is stored.
	listed := e.tokens(c)
	if len(listed) != 1 || listed[0].Id != created.Token.Id {
		t.Fatalf("tokens = %+v", listed)
	}
	var stored string
	if err := e.db.Read.QueryRow(`SELECT secret_hash FROM api_tokens`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == created.Secret || strings.Contains(stored, "grower_") {
		t.Errorf("stored %q, want a hash", stored)
	}

	if resp := e.withBearer(created.Secret, http.MethodGet, haPath); resp.StatusCode != http.StatusOK {
		t.Fatalf("summary with token: status %d", resp.StatusCode)
	}
	if used := e.tokens(c)[0].LastUsedAt; used == nil || !used.Equal(e.clock.t) {
		t.Errorf("last_used_at = %v, want %v", used, e.clock.t)
	}

	// Last use is written at most once an hour.
	first := e.clock.t
	e.clock.t = e.clock.t.Add(30 * time.Minute)
	e.withBearer(created.Secret, http.MethodGet, haPath)
	if used := e.tokens(c)[0].LastUsedAt; !used.Equal(first) {
		t.Errorf("last_used_at after 30 min = %v, want unchanged %v", used, first)
	}
	e.clock.t = e.clock.t.Add(31 * time.Minute)
	e.withBearer(created.Secret, http.MethodGet, haPath)
	if used := e.tokens(c)[0].LastUsedAt; !used.Equal(e.clock.t) {
		t.Errorf("last_used_at after an hour = %v, want %v", used, e.clock.t)
	}

	// Revoking stops the token at once.
	if resp := e.call(c, http.MethodDelete, "/api/v1/api-tokens/"+created.Token.Id, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke: status %d", resp.StatusCode)
	}
	resp := e.withBearer(created.Secret, http.MethodGet, haPath)
	if got := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
		t.Errorf("WWW-Authenticate = %q", got)
	}
	expectProblem(t, resp, http.StatusUnauthorized)
	expectProblem(t, e.call(c, http.MethodDelete, "/api/v1/api-tokens/"+created.Token.Id, ""), http.StatusNotFound)
}

func TestAPITokenValidationAndOwnership(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()

	for _, body := range []string{`{"label":"   "}`, `{"label":"` + strings.Repeat("x", 65) + `"}`} {
		expectProblem(t, e.call(c, http.MethodPost, "/api/v1/api-tokens", body), http.StatusBadRequest)
	}
	// Managing tokens needs the session: no anonymous access, and no token can mint more.
	expectProblem(t, e.call(nil, http.MethodGet, "/api/v1/api-tokens", ""), http.StatusUnauthorized)
	tok := e.newToken(c, "HA")
	expectProblem(t, e.withBearer(tok.Secret, http.MethodGet, "/api/v1/api-tokens"), http.StatusUnauthorized)

	// Another user sees none of these and can't revoke them.
	ivy := e.otherUser()
	if got := e.tokens(ivy); len(got) != 0 {
		t.Errorf("other user's tokens = %+v", got)
	}
	expectProblem(t, e.call(ivy, http.MethodDelete, "/api/v1/api-tokens/"+tok.Token.Id, ""), http.StatusNotFound)
	if got := e.tokens(c); len(got) != 1 {
		t.Errorf("owner's tokens after other user's revoke = %+v", got)
	}

	// Deleting the user deletes their tokens.
	if _, err := e.db.Write.Exec(`DELETE FROM users WHERE username = ?`, testUser); err != nil {
		t.Fatal(err)
	}
	expectProblem(t, e.withBearer(tok.Secret, http.MethodGet, haPath), http.StatusUnauthorized)
}

// A token reads the Home Assistant summary and nothing else.
func TestAPITokenScopedToIntegration(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	tok := e.newToken(c, "HA")
	p := e.newPlant(c, "Fern")

	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/today"},
		{http.MethodGet, "/api/v1/me"},
		{http.MethodGet, "/api/v1/plants"},
		{http.MethodGet, "/api/v1/export"},
		{http.MethodDelete, "/api/v1/plants/" + p.Id},
	} {
		resp := e.do(request{method: tt.method, path: tt.path, headers: map[string]string{"Authorization": "Bearer " + tok.Secret}})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s with token: status %d, want 401", tt.method, tt.path, resp.StatusCode)
		}
	}
	for _, header := range []string{"Bearer nope", "Bearer grower_nope", "Basic " + tok.Secret, tok.Secret} {
		resp := e.do(request{path: haPath, headers: map[string]string{"Authorization": header}})
		expectProblem(t, resp, http.StatusUnauthorized)
	}
	// With neither a cookie nor a token, the route asks for a bearer token.
	resp := e.call(nil, http.MethodGet, haPath, "")
	if got := resp.Header.Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}
	expectProblem(t, resp, http.StatusUnauthorized)
	// The session cookie works too, for checking the JSON in a browser.
	expectJSON[apigen.HomeAssistantSummary](t, e.call(c, http.MethodGet, haPath, ""), http.StatusOK)
}

func TestHomeAssistantSummary(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	tok := e.newToken(c, "HA")
	summary := func() apigen.HomeAssistantSummary {
		t.Helper()
		return expectJSON[apigen.HomeAssistantSummary](t, e.withBearer(tok.Secret, http.MethodGet, haPath), http.StatusOK)
	}

	empty := summary()
	if empty.Date.String() != "2026-05-01" || empty.NeedsCare != 0 || empty.Title != "" || empty.Message != "" || empty.Items == nil || len(empty.Items) != 0 {
		t.Errorf("empty summary = %+v", empty)
	}

	// Today is 2026-05-01. Two plants share a name, so their rooms tell them apart.
	office := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Office"}`), http.StatusCreated)
	living := expectJSON[apigen.Location](t, e.call(c, http.MethodPost, "/api/v1/locations", `{"name":"Living room"}`), http.StatusCreated)
	newPlantIn := func(name, loc string) apigen.Plant {
		return expectJSON[apigen.Plant](t, e.call(c, http.MethodPost, "/api/v1/plants", `{"name":"`+name+`","location_id":"`+loc+`"}`), http.StatusCreated)
	}
	officePothos, livingPothos, fern := newPlantIn("Pothos", office.Id), newPlantIn("Pothos", living.Id), e.newPlant(c, "Fern")
	e.newTask(c, officePothos.Id, `{"kind":"water","interval_days":7,"start_on":"2026-04-28"}`)
	e.newTask(c, livingPothos.Id, `{"kind":"water","interval_days":7,"start_on":"2026-05-01"}`)
	e.newTask(c, fern.Id, `{"kind":"mist","interval_days":7,"start_on":"2026-05-01"}`)
	e.newTask(c, fern.Id, `{"kind":"fertilize","interval_days":30,"start_on":"2026-05-04"}`)

	got := summary()
	if got.NeedsCare != 3 || got.Overdue != 1 || got.Due != 2 || got.Upcoming != 1 || got.Plants != 3 {
		t.Errorf("counts = needs %d, overdue %d, due %d, upcoming %d, plants %d", got.NeedsCare, got.Overdue, got.Due, got.Upcoming, got.Plants)
	}
	if got.Title != "3 plants need care" || got.Message != "Water Pothos (Office), Mist Fern and Water Pothos (Living room)" {
		t.Errorf("title, message = %q, %q", got.Title, got.Message)
	}
	var lines []string
	for _, it := range got.Items {
		room := "-"
		if it.Location != nil {
			room = *it.Location
		}
		lines = append(lines, it.Plant+"/"+room+"/"+it.Task+"/"+it.Label)
		if it.DaysOverdue < 0 || it.PlantId == "" || it.TaskId == "" {
			t.Errorf("item = %+v", it)
		}
	}
	want := "Pothos/Office/water/Water Pothos (Office)|Fern/-/mist/Mist Fern|Pothos/Living room/water/Water Pothos (Living room)"
	if s := strings.Join(lines, "|"); s != want {
		t.Errorf("items = %s\nwant    %s", s, want)
	}
	if got.Items[0].DaysOverdue != 3 {
		t.Errorf("overdue item days = %d, want 3", got.Items[0].DaysOverdue)
	}
}
