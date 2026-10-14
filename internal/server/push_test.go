package server

import (
	"net/http"
	"slices"
	"testing"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/push/pushtest"
)

func TestPushSubscriptions(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p256dh, auth := pushtest.Keys(t)
	sub := func(endpoint, p256dh, auth string) *http.Response {
		return e.call(c, http.MethodPost, "/api/v1/push/subscriptions",
			`{"endpoint":"`+endpoint+`","keys":{"p256dh":"`+p256dh+`","auth":"`+auth+`"}}`)
	}
	sendTest := func() {
		t.Helper()
		if resp := e.call(c, http.MethodPost, "/api/v1/push/test", ""); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("test push: status %d", resp.StatusCode)
		}
	}

	key := expectJSON[apigen.VapidKey](t, e.call(c, http.MethodGet, "/api/v1/push/vapid-key", ""), http.StatusOK)
	if len(key.PublicKey) < 80 {
		t.Errorf("public key = %q", key.PublicKey)
	}

	for _, endpoint := range []string{"http://push.example/x", "ftp://push.example/x", "not a url", ""} {
		if got := expectProblem(t, sub(endpoint, p256dh, auth), http.StatusBadRequest); got != "Endpoint must be an https URL." {
			t.Errorf("%q: detail = %q", endpoint, got)
		}
	}
	if got := expectProblem(t, sub(e.pushService.URL+"/a", "", auth), http.StatusBadRequest); got != "Subscription keys are required." {
		t.Errorf("no keys: detail = %q", got)
	}

	sendTest()    // no devices: nothing to do
	for range 2 { // subscribing again is an update, not a second device
		if resp := sub(e.pushService.URL+"/a", p256dh, auth); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("subscribe: status %d", resp.StatusCode)
		}
	}
	sendTest()
	if got := e.pushService.Requests(); !slices.Equal(got, []string{"/a"}) {
		t.Errorf("pushes = %v", got)
	}

	// A device the push service has forgotten is removed; a test with no device left
	// reaching anyone is still a 204.
	e.pushService.SetStatus("/a", http.StatusGone)
	sendTest()
	e.pushService.SetStatus("/a", http.StatusCreated)
	sendTest()
	if got := len(e.pushService.Requests()); got != 2 {
		t.Errorf("%d pushes after the device was gone, want 2", got)
	}

	// A failing push service makes the test a 500, so Settings can say so.
	sub(e.pushService.URL+"/b", p256dh, auth)
	e.pushService.SetStatus("/b", http.StatusInternalServerError)
	expectProblem(t, e.call(c, http.MethodPost, "/api/v1/push/test", ""), http.StatusInternalServerError)

	for range 2 { // unknown endpoints are fine too
		if resp := e.call(c, http.MethodDelete, "/api/v1/push/subscriptions", `{"endpoint":"`+e.pushService.URL+`/b"}`); resp.StatusCode != http.StatusNoContent {
			t.Fatalf("unsubscribe: status %d", resp.StatusCode)
		}
	}
	sendTest()

	expectProblem(t, e.call(nil, http.MethodGet, "/api/v1/push/vapid-key", ""), http.StatusUnauthorized)
	expectProblem(t, e.call(nil, http.MethodPost, "/api/v1/push/subscriptions", `{"endpoint":"https://x/y","keys":{"p256dh":"a","auth":"b"}}`), http.StatusUnauthorized)
	expectProblem(t, e.call(nil, http.MethodPost, "/api/v1/push/test", ""), http.StatusUnauthorized)
}
