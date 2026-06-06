package health

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/adkhorst/planbot/database"
)

// Status represents the health status of the application
type Status struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Database  string    `json:"database"`
}

// Server represents the health check HTTP server
type Server struct {
	port    string
	version string
}

// NewServer creates a new health check server
func NewServer(port, version string) *Server {
	return &Server{
		port:    port,
		version: version,
	}
}

// Start starts the health check HTTP server
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ready", s.readyHandler)
	mux.HandleFunc("/", s.rootHandler)

	addr := fmt.Sprintf(":%s", s.port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Health check server starting on %s", addr)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health check server error: %v", err)
		}
	}()
}

// healthHandler handles /health endpoint (liveness probe)
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	status := Status{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   s.version,
		Database:  "unknown",
	}

	// Check database connection
	if database.DB != nil {
		if err := database.DB.Ping(); err != nil {
			status.Status = "unhealthy"
			status.Database = "disconnected"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			status.Database = "connected"
			w.WriteHeader(http.StatusOK)
		}
	} else {
		status.Status = "unhealthy"
		status.Database = "not initialized"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	writeJSON(w, status)
}

// readyHandler handles /ready endpoint (readiness probe)
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	// Check if database is ready
	if database.DB == nil {
		writeText(w, http.StatusServiceUnavailable, "Database not initialized")
		return
	}

	if err := database.DB.Ping(); err != nil {
		writeText(w, http.StatusServiceUnavailable, "Database not ready")
		return
	}

	writeText(w, http.StatusOK, "Ready")
}

// rootHandler handles / endpoint
func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	response := map[string]string{
		"service": "PlanBot",
		"version": s.version,
		"status":  "running",
	}

	writeJSON(w, response)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode json: %v", err)
	}
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		log.Printf("write response: %v", err)
	}
}
