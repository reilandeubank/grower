package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/push"
)

// GetVapidKey returns the public key browsers subscribe with.
func (h *Handler) GetVapidKey(_ context.Context, _ apigen.GetVapidKeyRequestObject) (apigen.GetVapidKeyResponseObject, error) {
	return apigen.GetVapidKey200JSONResponse{PublicKey: h.push.Keys.Public}, nil
}

// SubscribePush registers a device for the daily reminder, or updates its keys.
func (h *Handler) SubscribePush(ctx context.Context, req apigen.SubscribePushRequestObject) (apigen.SubscribePushResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apigen.SubscribePush401ApplicationProblemPlusJSONResponse(unauthorized()), nil
	}
	b := req.Body
	// The server POSTs to the endpoint, so only a push service over https is accepted.
	if u, err := url.Parse(b.Endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
		return apigen.SubscribePush400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Endpoint must be an https URL.")}, nil
	}
	if b.Keys.P256dh == "" || b.Keys.Auth == "" {
		return apigen.SubscribePush400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Subscription keys are required.")}, nil
	}
	if err := h.db.Writer().UpsertPushSubscription(ctx, dbgen.UpsertPushSubscriptionParams{
		ID: db.NewID(), UserID: p.UserID, Endpoint: b.Endpoint, P256dh: b.Keys.P256dh, Auth: b.Keys.Auth,
		CreatedAt: db.FormatTime(h.auth.Now()),
	}); err != nil {
		return nil, fmt.Errorf("save push subscription: %w", err)
	}
	return apigen.SubscribePush204Response{}, nil
}

// UnsubscribePush forgets one of the user's devices. Unknown endpoints are ignored.
func (h *Handler) UnsubscribePush(ctx context.Context, req apigen.UnsubscribePushRequestObject) (apigen.UnsubscribePushResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apigen.UnsubscribePush401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: unauthorized()}, nil
	}
	if err := h.db.Writer().DeleteUserPushSubscription(ctx, dbgen.DeleteUserPushSubscriptionParams{UserID: p.UserID, Endpoint: req.Body.Endpoint}); err != nil {
		return nil, fmt.Errorf("delete push subscription: %w", err)
	}
	return apigen.UnsubscribePush204Response{}, nil
}

// SendTestPush notifies every device of the user now. A failure is a 500, so Settings
// can say the test did not arrive.
func (h *Handler) SendTestPush(ctx context.Context, _ apigen.SendTestPushRequestObject) (apigen.SendTestPushResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apigen.SendTestPush401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: unauthorized()}, nil
	}
	if err := h.push.Send(ctx, p.UserID, push.TestMessage); err != nil {
		return nil, fmt.Errorf("send test push: %w", err)
	}
	return apigen.SendTestPush204Response{}, nil
}
