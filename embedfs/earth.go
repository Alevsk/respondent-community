// Package embedfs provides the embedded frontend filesystem for the community binary.
//
// Build requirement: before running `go build ./cmd/community/`, the Earth frontend
// must be built and its dist output placed at embedfs/frontend/apps/earth/dist/.
// The Makefile target `build-community` handles this automatically:
//
//	make build-community
//
// Manual steps:
//
//	cd frontend && pnpm install && pnpm --filter earth build
//	cp -r frontend/apps/earth/dist embedfs/frontend/apps/earth/dist
//
// The //go:embed directive path is relative to this file's directory (embedfs/),
// so the full path on disk is embedfs/frontend/apps/earth/dist.
package embedfs

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/apps/earth/dist
var earthFS embed.FS

// EarthDistFS returns the embedded Earth frontend as an fs.FS rooted at
// frontend/apps/earth/dist, ready to be assigned to ui.EarthDist.
// Returns nil if the embedded filesystem cannot be sub-rooted (should not happen
// in a correctly built binary).
func EarthDistFS() fs.FS {
	sub, err := fs.Sub(earthFS, "frontend/apps/earth/dist")
	if err != nil {
		return nil
	}
	return sub
}
