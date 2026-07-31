// Package dashboard provides a Datastar-powered web dashboard for the frao-advisor MCP server.
// Runs as a goroutine alongside the MCP server on the configured port.
package dashboard

import (
	"encoding/csv"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/prhymera/frao-advisor/db"
)

//go:embed templates/layout.gohtml static/style.css static/datastar.js static/chart.umd.min.js static/dashboard.js
var contentFS embed.FS

// Start launches the dashboard HTTP server in a background goroutine.
// Returns the *http.Server so the caller can shut it down.
func getBindAddr() string {
	if h := os.Getenv("ADVISOR_DASHBOARD_HOST"); h != "" {
		return h
	}
	return "127.0.0.1" // security-by-default; override via ADVISOR_DASHBOARD_HOST
}
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

	// SSE routes (7 views)
	mux.HandleFunc("GET /dashboard/overview", h.Overview)
	mux.HandleFunc("GET /dashboard/timeline", h.Timeline)
	mux.HandleFunc("GET /dashboard/experts", h.Experts)
	mux.HandleFunc("GET /dashboard/deliberations", h.Deliberations)
	mux.HandleFunc("GET /dashboard/costs", h.Costs)
	mux.HandleFunc("GET /dashboard/metrics", h.Metrics)
	mux.HandleFunc("GET /dashboard/detail", h.Detail)
	// CSV export
	mux.HandleFunc("GET /export/csv", h.CSVExport)

	srv := &http.Server{
		Addr:    getBindAddr() + ":" + port,
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

// CSVExport streams all advice records as a CSV download.
func (h *Handlers) CSVExport(w http.ResponseWriter, r *http.Request) {
	entries, _, err := h.DB.Timeline(r.Context(), "", "", 50000, 0)
	if err != nil {
		http.Error(w, "Failed to fetch data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=frao-advisor-export.csv")

	wr := csv.NewWriter(w)
	defer wr.Flush()

	// Header row
	wr.Write([]string{"Date", "Type", "Expert", "Model", "Tokens", "Cost", "Duration (ms)"})

	for _, e := range entries {
		wr.Write([]string{
			sanitizeCSVField(e.CreatedAt),
			sanitizeCSVField(e.Type),
			sanitizeCSVField(e.ExpertKey),
			sanitizeCSVField(e.Model),
			fmt.Sprintf("%d", e.Tokens),
			fmt.Sprintf("%.6f", e.Cost),
			fmt.Sprintf("%d", e.DurationMs),
		})
	}
}

// sanitizeCSVField prefixes dangerous leading characters to prevent CSV injection
// in spreadsheet applications (e.g., formulas starting with =, +, -, @).
func sanitizeCSVField(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}
