// Package web embeds the built frontend (web/dist) into the Go binary.
package web

import (
	"embed"
	"io/fs"
)

// dist is populated by `npm --prefix web run build`. The committed dist/.gitkeep keeps
// this compiling before a frontend build; the server then reports the missing build.
//
//go:embed all:dist
var dist embed.FS

// FS returns the contents of web/dist with the "dist/" prefix removed.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// fs.Sub only fails on an invalid path literal, which "dist" is not.
		panic(err)
	}
	return sub
}
