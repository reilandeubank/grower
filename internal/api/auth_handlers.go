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
	"github.com/reilandeubank/grower/internal/buildinfo"
)

// Login checks credentials and sets the session cookie.
func (h *Handler) Login(ctx context.Context, req apigen.LoginRequestObject) (apigen.LoginResponseObject, error) {
	body := req.Body
	if n := utf8.RuneCountInString(body.Username); n < 1 || n > 64 || body.Password == "" || utf8.RuneCountInString(body.Password) > 256 {
		return apigen.Login400ApplicationProblemPlusJSONResponse{
			ProblemApplicationProblemPlusJSONResponse: badRequest("Username and password are required."),
		}, nil
	}

	r := requestFrom(ctx)
	principal, token, err := h.auth.Login(ctx, body.Username, body.Password, clientIP(r), r.UserAgent())
	switch {
	case errors.Is(err, auth.ErrRateLimited):
		return apigen.Login429ApplicationProblemPlusJSONResponse(
			problem(http.StatusTooManyRequests, "Too many login attempts. Wait a minute and try again.")), nil
	case errors.Is(err, auth.ErrInvalidCredentials):
		return apigen.Login401ApplicationProblemPlusJSONResponse(
			problem(http.StatusUnauthorized, "Wrong username or password.")), nil
	case err != nil:
		return nil, fmt.Errorf("login: %w", err)
	}

	me, err := h.me(ctx, principal)
	if err != nil {
		return nil, err
	}
	return loginSuccess{me: me, cookie: h.cookie, token: token, expires: principal.ExpiresAt, now: h.auth.Now()}, nil
}

// loginSuccess wraps the generated 200 response to also set the session cookie.
type loginSuccess struct {
	me      apigen.Me
	cookie  auth.CookieConfig
	token   string
	expires time.Time
	now     time.Time
}

func (l loginSuccess) VisitLoginResponse(w http.ResponseWriter) error {
	l.cookie.Set(w, l.token, l.expires, l.now)
	return apigen.Login200JSONResponse(l.me).VisitLoginResponse(w)
}

// Logout revokes the current session (if any) and clears the cookie.
func (h *Handler) Logout(ctx context.Context, _ apigen.LogoutRequestObject) (apigen.LogoutResponseObject, error) {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		if err := h.auth.Sessions().Revoke(ctx, p.SessionHash); err != nil {
			return nil, fmt.Errorf("logout: %w", err)
		}
	}
	return logoutResponse{cookie: h.cookie}, nil
}

type logoutResponse struct{ cookie auth.CookieConfig }

func (l logoutResponse) VisitLogoutResponse(w http.ResponseWriter) error {
	l.cookie.Clear(w)
	return apigen.Logout204Response{}.VisitLogoutResponse(w)
}

// GetMe returns the signed-in user, their settings and the server version.
func (h *Handler) GetMe(ctx context.Context, _ apigen.GetMeRequestObject) (apigen.GetMeResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apigen.GetMe401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: unauthorized()}, nil
	}
	me, err := h.me(ctx, p)
	if err != nil {
		return nil, err
	}
	return apigen.GetMe200JSONResponse(me), nil
}

func (h *Handler) me(ctx context.Context, p auth.Principal) (apigen.Me, error) {
	settings, err := h.db.Reader().GetSettings(ctx, p.UserID)
	if err != nil {
		return apigen.Me{}, fmt.Errorf("load settings for user %s: %w", p.UserID, err)
	}
	return apigen.Me{
		User:     apigen.User{Id: p.UserID, Username: p.Username},
		Settings: settingsOf(settings),
		Version:  buildinfo.Version,
	}, nil
}
