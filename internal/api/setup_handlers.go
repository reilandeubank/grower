package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
)

// GetSetup reports whether the first account still needs creating. It is public and
// says nothing else, so it reveals no more than the login page does.
func (h *Handler) GetSetup(ctx context.Context, _ apigen.GetSetupRequestObject) (apigen.GetSetupResponseObject, error) {
	needed, err := h.auth.SetupNeeded(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetSetup200JSONResponse{Needed: needed}, nil
}

// CompleteSetup creates the first account with the setup code and signs it in.
func (h *Handler) CompleteSetup(ctx context.Context, req apigen.CompleteSetupRequestObject) (apigen.CompleteSetupResponseObject, error) {
	body := req.Body
	if body.Code == "" || utf8.RuneCountInString(body.Code) > 128 || body.Username == "" || body.Password == "" || utf8.RuneCountInString(body.Password) > 256 {
		return apigen.CompleteSetup400ApplicationProblemPlusJSONResponse{
			ProblemApplicationProblemPlusJSONResponse: badRequest("Setup code, username and password are required."),
		}, nil
	}

	r := requestFrom(ctx)
	principal, token, err := h.auth.CompleteSetup(ctx, body.Code, body.Username, body.Password, h.setupZone(body.Timezone), clientIP(r), r.UserAgent())
	switch {
	case errors.Is(err, auth.ErrRateLimited):
		return apigen.CompleteSetup429ApplicationProblemPlusJSONResponse(
			problem(http.StatusTooManyRequests, "Too many attempts. Wait a minute and try again.")), nil
	case errors.Is(err, auth.ErrSetupDone):
		return apigen.CompleteSetup409ApplicationProblemPlusJSONResponse(
			problem(http.StatusConflict, "This grower already has an account. Log in instead.")), nil
	case errors.Is(err, auth.ErrWrongSetupCode):
		return apigen.CompleteSetup403ApplicationProblemPlusJSONResponse(
			problem(http.StatusForbidden, "That setup code doesn't match. Check the server log for the current one.")), nil
	case errors.Is(err, auth.ErrInvalidUsername):
		return apigen.CompleteSetup400ApplicationProblemPlusJSONResponse{
			ProblemApplicationProblemPlusJSONResponse: badRequest("Use a username of up to 64 characters with no spaces."),
		}, nil
	case errors.Is(err, auth.ErrPasswordTooShort):
		return apigen.CompleteSetup400ApplicationProblemPlusJSONResponse{
			ProblemApplicationProblemPlusJSONResponse: badRequest(fmt.Sprintf("Use a password of at least %d characters.", auth.MinPasswordLength)),
		}, nil
	case err != nil:
		return nil, fmt.Errorf("complete setup: %w", err)
	}

	me, err := h.me(ctx, principal)
	if err != nil {
		return nil, err
	}
	return loginSuccess{me: me, cookie: h.cookie, token: token, expires: principal.ExpiresAt, now: h.auth.Now()}, nil
}

// setupZone is the browser's zone when it is a real IANA name, else GROWER_TZ.
func (h *Handler) setupZone(tz *string) string {
	if tz == nil || *tz == "" || *tz == "Local" {
		return h.defaultZone
	}
	if _, err := time.LoadLocation(*tz); err != nil {
		return h.defaultZone
	}
	return *tz
}

// VisitCompleteSetupResponse lets setup reuse the login response, cookie included.
func (l loginSuccess) VisitCompleteSetupResponse(w http.ResponseWriter) error {
	l.cookie.Set(w, l.token, l.expires, l.now)
	return apigen.CompleteSetup200JSONResponse(l.me).VisitCompleteSetupResponse(w)
}
