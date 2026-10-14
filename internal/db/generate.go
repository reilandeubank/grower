package db

// CGO_ENABLED=0 keeps sqlc on its pure-Go path (no C toolchain needed for the SQLite engine).
//go:generate env CGO_ENABLED=0 go tool sqlc generate -f sqlc.yaml
