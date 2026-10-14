package api

import (
	"encoding/json"
	"net/http"

	"github.com/reilandeubank/grower/internal/apigen"
)

// problem builds an RFC 9457 problem with the standard status text as its title.
func problem(status int, detail string) apigen.Problem {
	p := apigen.Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	return p
}

// WriteProblem writes an application/problem+json response. It is used for errors
// raised outside the generated handlers (middleware, unknown routes).
func WriteProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem(status, detail))
}
