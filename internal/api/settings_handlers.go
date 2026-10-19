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
	"github.com/reilandeubank/grower/internal/backup"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/push"
)

// unauthorized is the backstop for handlers reached without a session; the server's
// auth middleware normally answers first.
func unauthorized() apigen.ProblemApplicationProblemPlusJSONResponse {
	return apigen.ProblemApplicationProblemPlusJSONResponse(problem(http.StatusUnauthorized, "Sign in to continue."))
}

// UpdateSettings replaces the user's settings. Today follows a new time zone at once,
// since it reads the zone on every request.
func (h *Handler) UpdateSettings(ctx context.Context, req apigen.UpdateSettingsRequestObject) (apigen.UpdateSettingsResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apigen.UpdateSettings401ApplicationProblemPlusJSONResponse(unauthorized()), nil
	}
	tz := req.Body.Timezone
	// LoadLocation maps "" to UTC and "Local" to the server's zone; neither is a choice
	// from the list.
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" {
		return apigen.UpdateSettings400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Pick a time zone from the list.")}, nil
	}
	if _, err := push.ParseClock(req.Body.DigestTime); err != nil {
		return apigen.UpdateSettings400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Pick a time like 08:00.")}, nil
	}
	if !req.Body.Hemisphere.Valid() {
		return apigen.UpdateSettings400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Hemisphere must be north or south.")}, nil
	}
	row, err := h.db.Writer().UpdateSettings(ctx, dbgen.UpdateSettingsParams{
		Timezone: tz, DigestLocalTime: req.Body.DigestTime, Hemisphere: string(req.Body.Hemisphere), UpdatedAt: db.FormatTime(h.auth.Now()), UserID: p.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("update settings for user %s: %w", p.UserID, err)
	}
	return apigen.UpdateSettings200JSONResponse(settingsOf(row)), nil
}

func settingsOf(row dbgen.Setting) apigen.Settings {
	return apigen.Settings{Timezone: row.Timezone, DigestTime: row.DigestLocalTime, Hemisphere: apigen.SettingsHemisphere(row.Hemisphere)}
}

// ChangePassword sets a new password after checking the current one, keeping only
// this session signed in.
func (h *Handler) ChangePassword(ctx context.Context, req apigen.ChangePasswordRequestObject) (apigen.ChangePasswordResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apigen.ChangePassword401ApplicationProblemPlusJSONResponse(unauthorized()), nil
	}
	if utf8.RuneCountInString(req.Body.CurrentPassword) > 256 || utf8.RuneCountInString(req.Body.NewPassword) > 256 {
		return apigen.ChangePassword400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Passwords must be 256 characters or fewer.")}, nil
	}
	err := h.auth.ChangePassword(ctx, p, req.Body.CurrentPassword, req.Body.NewPassword, clientIP(requestFrom(ctx)))
	switch {
	case errors.Is(err, auth.ErrPasswordTooShort):
		return apigen.ChangePassword400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Use at least 10 characters.")}, nil
	case errors.Is(err, auth.ErrRateLimited):
		return apigen.ChangePassword429ApplicationProblemPlusJSONResponse(
			problem(http.StatusTooManyRequests, "Too many attempts. Wait a minute and try again.")), nil
	case errors.Is(err, auth.ErrWrongPassword):
		return apigen.ChangePassword400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Your current password is wrong.")}, nil
	case err != nil:
		return nil, fmt.Errorf("change password: %w", err)
	}
	return apigen.ChangePassword204Response{}, nil
}

// GetStorage reports disk use and when the last nightly backup was written.
func (h *Handler) GetStorage(ctx context.Context, _ apigen.GetStorageRequestObject) (apigen.GetStorageResponseObject, error) {
	count, err := h.db.Reader().CountPhotos(ctx)
	if err != nil {
		return nil, fmt.Errorf("count photos: %w", err)
	}
	photoBytes, err := h.media.Size()
	if err != nil {
		return nil, err
	}
	dbBytes, err := h.db.Size()
	if err != nil {
		return nil, fmt.Errorf("database size: %w", err)
	}
	res := apigen.Storage{PhotoCount: int(count), PhotoBytes: photoBytes, DatabaseBytes: dbBytes}
	last, ok, err := backup.Latest(h.dataDir)
	if err != nil {
		return nil, err
	}
	if ok {
		res.LastBackupAt = &last
	}
	return apigen.GetStorage200JSONResponse(res), nil
}
