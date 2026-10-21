package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/species"
)

const (
	identifyLimit   = "Plant identification's daily limit is used up. Try again tomorrow."
	identifyNoMatch = "No match found. Try a clearer photo of a leaf or flower."
	identifyFailed  = "Couldn't reach Pl@ntNet. Try again later."
)

// IdentifyPlant forwards a photo to Pl@ntNet and returns its best suggestions, marking
// species already saved. The photo is never stored.
func (h *Handler) IdentifyPlant(ctx context.Context, req apigen.IdentifyPlantRequestObject) (apigen.IdentifyPlantResponseObject, error) {
	out := apigen.Identification{Configured: h.identify != nil, Candidates: []apigen.IdentifyCandidate{}}
	if h.identify == nil {
		return apigen.IdentifyPlant200JSONResponse(out), nil
	}
	bad := func(detail string) (apigen.IdentifyPlantResponseObject, error) {
		return apigen.IdentifyPlant400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	data, organ, err := readIdentify(req.Body)
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return apigen.IdentifyPlant413ApplicationProblemPlusJSONResponse(problem(http.StatusRequestEntityTooLarge, photoTooLarge)), nil
	case err != nil:
		return bad("The upload is not valid multipart form data.")
	case data == nil:
		return bad("Choose a photo to identify.")
	case !organ.Valid():
		return bad("Organ must be auto, leaf, flower, fruit or bark.")
	}
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" {
		return bad("Only JPEG and PNG photos are supported.")
	}

	found, err := h.identify.Identify(ctx, data, mime, string(organ))
	switch {
	case errors.Is(err, species.ErrDailyLimit):
		out.Detail = identifyLimit
		return apigen.IdentifyPlant200JSONResponse(out), nil
	case errors.Is(err, species.ErrNoMatch):
		out.Detail = identifyNoMatch
		return apigen.IdentifyPlant200JSONResponse(out), nil
	case err != nil:
		h.log.WarnContext(ctx, "plant identification failed", "err", err)
		return apigen.IdentifyPlant502ApplicationProblemPlusJSONResponse(problem(http.StatusBadGateway, identifyFailed)), nil
	}

	names, err := h.db.Reader().ListSpeciesNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list species names: %w", err)
	}
	saved := make(map[string]string, len(names))
	for _, n := range names {
		saved[strings.ToLower(n.ScientificName)] = n.ID
	}
	for _, f := range found {
		c := apigen.IdentifyCandidate{ScientificName: f.ScientificName, CommonName: f.CommonName, Score: f.Score}
		if id, ok := saved[strings.ToLower(f.ScientificName)]; ok {
			c.SavedId = &id
		}
		out.Candidates = append(out.Candidates, c)
	}
	return apigen.IdentifyPlant200JSONResponse(out), nil
}

// readIdentify reads the file and organ fields. A read error (including
// *http.MaxBytesError) is returned as is.
func readIdentify(r *multipart.Reader) ([]byte, apigen.IdentifyRequestOrgan, error) {
	var data []byte
	organ := apigen.Auto
	for {
		part, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			return data, organ, nil
		}
		if err != nil {
			return nil, organ, err
		}
		b, err := io.ReadAll(part)
		if err != nil {
			return nil, organ, err
		}
		switch part.FormName() {
		case "file":
			data = b
		case "organ":
			organ = apigen.IdentifyRequestOrgan(b)
		}
	}
}
