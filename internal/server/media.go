package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/reilandeubank/grower/internal/api"
	"github.com/reilandeubank/grower/internal/media"
)

const photoNotFound = "Photo not found."

// mediaHandler serves GET /media/photos/{id}/{size}. A photo's files never change once
// written, so they are cached for good, but only privately: they are personal and the
// route needs a session.
func mediaHandler(store *media.Store, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, name, err := store.Open(r.PathValue("id"), r.PathValue("size"))
		switch {
		case errors.Is(err, media.ErrNotFound):
			api.WriteProblem(w, http.StatusNotFound, photoNotFound)
			return
		case err != nil:
			log.ErrorContext(r.Context(), "open photo", "path", r.URL.Path, "err", err)
			api.WriteProblem(w, http.StatusInternalServerError, "")
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			log.ErrorContext(r.Context(), "stat photo", "path", r.URL.Path, "err", err)
			api.WriteProblem(w, http.StatusInternalServerError, "")
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		// The extension of name (.jpg or .png) sets Content-Type.
		http.ServeContent(w, r, name, info.ModTime(), f)
	})
}
