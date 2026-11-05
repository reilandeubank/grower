package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
)

const maxTokenLabel = 64

// ListApiTokens lists the signed-in user's API tokens.
func (h *Handler) ListApiTokens(ctx context.Context, _ apigen.ListApiTokensRequestObject) (apigen.ListApiTokensResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, errors.New("no signed-in user")
	}
	tokens, err := h.auth.Tokens().List(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	res := apigen.ApiTokenList{Items: make([]apigen.ApiToken, len(tokens))}
	for i, t := range tokens {
		res.Items[i] = apiToken(t)
	}
	return apigen.ListApiTokens200JSONResponse(res), nil
}

// CreateApiToken makes a read-only token and returns its secret, once.
func (h *Handler) CreateApiToken(ctx context.Context, req apigen.CreateApiTokenRequestObject) (apigen.CreateApiTokenResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, errors.New("no signed-in user")
	}
	label := strings.TrimSpace(req.Body.Label)
	switch {
	case label == "":
		return apigen.CreateApiToken400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Name the token, for example after the app that will use it.")}, nil
	case utf8.RuneCountInString(label) > maxTokenLabel:
		return apigen.CreateApiToken400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Use a name of at most 64 characters.")}, nil
	}
	t, secret, err := h.auth.Tokens().Create(ctx, p.UserID, label)
	if err != nil {
		return nil, err
	}
	return apigen.CreateApiToken201JSONResponse{Token: apiToken(t), Secret: secret}, nil
}

// DeleteApiToken revokes one of the signed-in user's tokens.
func (h *Handler) DeleteApiToken(ctx context.Context, req apigen.DeleteApiTokenRequestObject) (apigen.DeleteApiTokenResponseObject, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, errors.New("no signed-in user")
	}
	found, err := h.auth.Tokens().Revoke(ctx, p.UserID, req.Id)
	if err != nil {
		return nil, err
	}
	if !found {
		return apigen.DeleteApiToken404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, "No such API token.")), nil
	}
	return apigen.DeleteApiToken204Response{}, nil
}

func apiToken(t auth.APIToken) apigen.ApiToken {
	return apigen.ApiToken{Id: t.ID, Label: t.Label, Scope: apigen.ApiTokenScope(t.Scope), CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt}
}
