// Package dashboard provides a Datastar-powered web dashboard for the frao-advisor MCP server.
// Runs as a goroutine alongside the MCP server on the configured port.
package dashboard

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/prhymera/frao-advisor/db"
)

//go:embed templates/layout.gohtml static/style.css
var contentFS embed.FS

// Start launches the dashboard HTTP server in a background goroutine.
// Returns the *http.Server so the caller can shut it down.
func Start(database *db.DB, port string) *http.Server {
	layout := template.Must(template.ParseFS(contentFS, "templates/layout.gohtml"))

	h := &Handlers{
		DB:     database,
		layout: layout,
	}

	mux := http.NewServeMux()

	// Static CSS (served from embedded static/ directory)
	staticSub, err := fs.Sub(contentFS, "static")
	if err != nil {
		log.Fatalf("dashboard static fs: %v", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Full HTML page
	mux.HandleFunc("GET /", h.LayoutPage)
	mux.HandleFunc("GET /dashboard", h.LayoutPage)

	// SSE routes (6 views)
	mux.HandleFunc("GET /dashboard/overview", h.Overview)
	mux.HandleFunc("GET /dashboard/timeline", h.Timeline)
	mux.HandleFunc("GET /dashboard/experts", h.Experts)
	mux.HandleFunc("GET /dashboard/deliberations", h.Deliberations)
	mux.HandleFunc("GET /dashboard/costs", h.Costs)
	mux.HandleFunc("GET /dashboard/metrics", h.Metrics)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("dashboard — http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("dashboard: %v", err)
		}
	}()

	return srv
}

// isDatastarReq checks whether the request comes from a Datastar client
// (SSE event stream) vs a direct browser navigation.
func isDatastarReq(r *http.Request) bool {
	return r.Header.Get("Datastar-Request") == "true" ||
		strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}
