package ui

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// EarthDist embeds the pre-built Earth frontend.
// This variable is populated by //go:embed in the community binary,
// or overridden with a real filesystem in development mode.
//
// In production, the community binary's main package sets this:
//
//	//go:embed all:frontend/apps/earth/dist
//	var earthFS embed.FS
//	ui.EarthDist, _ = fs.Sub(earthFS, "frontend/apps/earth/dist")
//
// In development/tests, pass os.DirFS("path/to/dist").
var EarthDist fs.FS

// NewHandler creates an http.Handler serving the Earth frontend.
// Falls back to index.html for paths that don't match a real file (SPA routing).
// If dist is nil, returns a handler that responds 503.
func NewHandler(dist fs.FS) http.Handler {
	if dist == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not embedded", http.StatusServiceUnavailable)
		})
	}
	fileServer := http.FileServer(http.FS(dist))
	return &spaHandler{fs: dist, fileServer: fileServer}
}

type spaHandler struct {
	fs         fs.FS
	fileServer http.Handler
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path and strip leading slash for fs.Open.
	p := path.Clean(r.URL.Path)
	if p == "/" {
		p = "index.html"
	} else {
		p = strings.TrimPrefix(p, "/")
	}

	// Check if the file exists in the embedded filesystem.
	f, err := h.fs.Open(p)
	if err == nil {
		_ = f.Close()
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// File not found — serve index.html for SPA client-side routing.
	indexFile, err := h.fs.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	defer func() { _ = indexFile.Close() }()

	stat, err := indexFile.Stat()
	if err != nil {
		http.Error(w, "cannot stat index.html", http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(readSeeker))
}

// readSeeker combines io.ReadSeeker for http.ServeContent.
type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}
