package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/media"
)

// MaxUploadBytes caps a photo upload request. The client resizes photos to at most
// 2560 px first, so real uploads are a few megabytes.
const MaxUploadBytes = 15 << 20

const (
	photoNotFound   = "Photo not found."
	photoTooLarge   = "Photos must be 15 MB or smaller."
	otherPlantPhoto = "That photo belongs to another plant."
)

// ListPlantPhotos returns a plant's photos, newest first.
func (h *Handler) ListPlantPhotos(ctx context.Context, req apigen.ListPlantPhotosRequestObject) (apigen.ListPlantPhotosResponseObject, error) {
	if _, err := h.db.Reader().GetPlant(ctx, req.Id); errors.Is(err, sql.ErrNoRows) {
		return apigen.ListPlantPhotos404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	} else if err != nil {
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	}
	rows, err := h.db.Reader().ListPlantPhotos(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("list photos of plant %s: %w", req.Id, err)
	}
	items := make([]apigen.Photo, len(rows))
	for i, r := range rows {
		if items[i], err = photo(r); err != nil {
			return nil, err
		}
	}
	return apigen.ListPlantPhotos200JSONResponse{Items: items}, nil
}

// UploadPhoto stores a photo and its thumbnails. Uploading the same bytes to the same
// plant again returns the existing photo. A plant's first photo becomes its cover.
func (h *Handler) UploadPhoto(ctx context.Context, req apigen.UploadPhotoRequestObject) (apigen.UploadPhotoResponseObject, error) {
	if _, err := h.db.Reader().GetPlant(ctx, req.Id); errors.Is(err, sql.ErrNoRows) {
		return apigen.UploadPhoto404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
	} else if err != nil {
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	}
	in, detail, err := readUpload(req.Body, h.auth.Now())
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return apigen.UploadPhoto413ApplicationProblemPlusJSONResponse(problem(http.StatusRequestEntityTooLarge, photoTooLarge)), nil
	case err != nil:
		detail = "The upload is not valid multipart form data."
	}
	if detail != "" {
		return apigen.UploadPhoto400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}

	sum := sha256.Sum256(in.data)
	hash := hex.EncodeToString(sum[:])
	if res, ok, err := h.duplicate(ctx, req.Id, hash); ok || err != nil {
		return res, err
	}

	id := db.NewID()
	img, err := h.media.Save(id, in.data)
	switch {
	case errors.Is(err, media.ErrUnsupported):
		return apigen.UploadPhoto400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Only JPEG and PNG photos are supported.")}, nil
	case errors.Is(err, media.ErrUnreadable):
		return apigen.UploadPhoto400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("That photo could not be read. It may be damaged.")}, nil
	case errors.Is(err, media.ErrTooLarge):
		return apigen.UploadPhoto400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest("Photos must be 50 megapixels or smaller.")}, nil
	case err != nil:
		return nil, fmt.Errorf("store photo: %w", err)
	}

	now := db.FormatTime(h.auth.Now())
	var row dbgen.Photo
	err = h.db.InTx(ctx, func(q *dbgen.Queries) error {
		var err error
		row, err = q.CreatePhoto(ctx, dbgen.CreatePhotoParams{
			ID: id, PlantID: req.Id, TakenAt: db.FormatTime(in.takenAt), Caption: in.caption, Mime: img.MIME,
			Width: int64(img.Width), Height: int64(img.Height), Bytes: int64(len(in.data)), Sha256: hash,
			CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		return q.SetCoverIfNone(ctx, dbgen.SetCoverIfNoneParams{PhotoID: sql.NullString{String: id, Valid: true}, PlantID: req.Id})
	})
	if err != nil {
		h.removeFiles(ctx, id)
		if db.IsForeignKeyViolation(err) { // the plant was deleted meanwhile
			return apigen.UploadPhoto404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound)), nil
		}
		if db.IsUniqueViolation(err) { // an identical upload finished first
			if res, ok, err := h.duplicate(ctx, req.Id, hash); ok || err != nil {
				return res, err
			}
		}
		return nil, fmt.Errorf("create photo: %w", err)
	}
	p, err := photo(row)
	if err != nil {
		return nil, err
	}
	return apigen.UploadPhoto201JSONResponse(p), nil
}

// duplicate returns the plant's existing photo with this hash as a 200, or ok=false if there is none.
func (h *Handler) duplicate(ctx context.Context, plantID, hash string) (apigen.UploadPhotoResponseObject, bool, error) {
	row, err := h.db.Reader().GetPlantPhotoByHash(ctx, dbgen.GetPlantPhotoByHashParams{PlantID: plantID, Sha256: hash})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find photo by hash: %w", err)
	}
	p, err := photo(row)
	if err != nil {
		return nil, false, err
	}
	return apigen.UploadPhoto200JSONResponse(p), true, nil
}

type upload struct {
	data    []byte
	caption string
	takenAt time.Time
}

// readUpload reads the multipart fields, returning a user-facing detail when they are
// invalid. A read error (including *http.MaxBytesError) is returned as is.
func readUpload(r *multipart.Reader, now time.Time) (upload, string, error) {
	in := upload{takenAt: now}
	var takenAt string
	for {
		part, err := r.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return in, "", err
		}
		b, err := io.ReadAll(part)
		if err != nil {
			return in, "", err
		}
		switch part.FormName() {
		case "file":
			in.data = b
		case "caption":
			in.caption = string(b)
		case "taken_at":
			takenAt = string(b)
		}
	}
	if in.data == nil {
		return in, "Choose a photo to upload.", nil
	}
	if takenAt != "" {
		t, err := time.Parse(time.RFC3339, takenAt)
		if err != nil {
			return in, "Taken at must be an RFC 3339 date and time.", nil
		}
		in.takenAt = t
	}
	var detail string
	in.caption, detail = parseCaption(in.caption)
	return in, detail, nil
}

func parseCaption(s string) (string, string) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 500 {
		return s, "Caption must be 500 characters or fewer."
	}
	return s, ""
}

// UpdatePhoto replaces a photo's caption and date.
func (h *Handler) UpdatePhoto(ctx context.Context, req apigen.UpdatePhotoRequestObject) (apigen.UpdatePhotoResponseObject, error) {
	caption, detail := parseCaption(req.Body.Caption)
	if detail == "" && req.Body.TakenAt.IsZero() {
		detail = "Taken at is required."
	}
	if detail != "" {
		return apigen.UpdatePhoto400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(detail)}, nil
	}
	row, err := h.db.Writer().UpdatePhoto(ctx, dbgen.UpdatePhotoParams{
		Caption: caption, TakenAt: db.FormatTime(req.Body.TakenAt), UpdatedAt: db.FormatTime(h.auth.Now()), ID: req.Id,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return apigen.UpdatePhoto404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, photoNotFound)), nil
	case err != nil:
		return nil, fmt.Errorf("update photo %s: %w", req.Id, err)
	}
	p, err := photo(row)
	if err != nil {
		return nil, err
	}
	return apigen.UpdatePhoto200JSONResponse(p), nil
}

// DeletePhoto removes a photo and its files. A deleted cover is replaced by the
// plant's newest remaining photo, if any.
func (h *Handler) DeletePhoto(ctx context.Context, req apigen.DeletePhotoRequestObject) (apigen.DeletePhotoResponseObject, error) {
	var n int64
	err := h.db.InTx(ctx, func(q *dbgen.Queries) error {
		if err := q.ReplaceCover(ctx, req.Id); err != nil {
			return err
		}
		var err error
		n, err = q.DeletePhoto(ctx, req.Id)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("delete photo %s: %w", req.Id, err)
	}
	if n == 0 {
		return apigen.DeletePhoto404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, photoNotFound)), nil
	}
	h.removeFiles(ctx, req.Id)
	return apigen.DeletePhoto204Response{}, nil
}

// SetPlantCover sets or, with a null photo_id, clears a plant's cover.
func (h *Handler) SetPlantCover(ctx context.Context, req apigen.SetPlantCoverRequestObject) (apigen.SetPlantCoverResponseObject, error) {
	notFound := apigen.SetPlantCover404ApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, plantNotFound))
	if _, err := h.db.Reader().GetPlant(ctx, req.Id); errors.Is(err, sql.ErrNoRows) {
		return notFound, nil
	} else if err != nil {
		return nil, fmt.Errorf("get plant %s: %w", req.Id, err)
	}
	var cover sql.NullString
	if id := req.Body.PhotoId; id != nil {
		ph, err := h.db.Reader().GetPhoto(ctx, *id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get photo %s: %w", *id, err)
		}
		if err != nil || ph.PlantID != req.Id {
			return apigen.SetPlantCover400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: badRequest(otherPlantPhoto)}, nil
		}
		cover = sql.NullString{String: *id, Valid: true}
	}
	row, err := h.db.Writer().SetPlantCover(ctx, dbgen.SetPlantCoverParams{CoverPhotoID: cover, UpdatedAt: db.FormatTime(h.auth.Now()), ID: req.Id})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return notFound, nil
	case err != nil:
		return nil, fmt.Errorf("set cover of plant %s: %w", req.Id, err)
	}
	p, err := h.plantOf(ctx, row)
	if err != nil {
		return nil, err
	}
	return apigen.SetPlantCover200JSONResponse(p), nil
}

// covers returns every plant's cover photo by plant id, in one query.
func (h *Handler) covers(ctx context.Context) (map[string]*dbgen.Photo, error) {
	rows, err := h.db.Reader().ListCoverPhotos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cover photos: %w", err)
	}
	m := make(map[string]*dbgen.Photo, len(rows))
	for i := range rows {
		m[rows[i].PlantID] = &rows[i]
	}
	return m, nil
}

// plantOf maps a single plant, loading its cover and room light.
func (h *Handler) plantOf(ctx context.Context, r dbgen.Plant) (apigen.Plant, error) {
	rooms, err := h.rooms(ctx)
	if err != nil {
		return apigen.Plant{}, err
	}
	if !r.CoverPhotoID.Valid {
		return plant(r, nil, rooms)
	}
	c, err := h.db.Reader().GetPhoto(ctx, r.CoverPhotoID.String)
	if err != nil {
		return apigen.Plant{}, fmt.Errorf("get cover of plant %s: %w", r.ID, err)
	}
	return plant(r, &c, rooms)
}

// removeFiles deletes photos' files after their rows are gone. A failure only leaves
// orphaned files behind, so it is logged rather than failing the request.
func (h *Handler) removeFiles(ctx context.Context, ids ...string) {
	for _, id := range ids {
		if err := h.media.Remove(id); err != nil {
			h.log.WarnContext(ctx, "remove photo files", "photo_id", id, "err", err)
		}
	}
}

func photo(r dbgen.Photo) (apigen.Photo, error) {
	taken, err := db.ParseTime(r.TakenAt)
	if err != nil {
		return apigen.Photo{}, err
	}
	base := "/media/photos/" + r.ID + "/"
	return apigen.Photo{
		Id: r.ID, PlantId: r.PlantID, Caption: r.Caption, TakenAt: taken,
		Width: int(r.Width), Height: int(r.Height),
		Urls: apigen.PhotoUrls{Small: base + "small", Medium: base + "medium", Original: base + "original"},
	}, nil
}
