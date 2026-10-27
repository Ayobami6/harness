package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Pinger abstracts database connectivity checks.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Response represents the structured JSON health status.
type Response struct {
	Status  string            `json:"status"`
	Details map[string]string `json:"details,omitempty"`
}

// NewHandler constructs an http.HandlerFunc for /healthz.
func NewHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		status := "ok"
		statusCode := http.StatusOK
		details := make(map[string]string)

		if db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()

			if err := db.PingContext(ctx); err != nil {
				status = "unavailable"
				statusCode = http.StatusServiceUnavailable
				details["database"] = "down"
			} else {
				details["database"] = "up"
			}
		}

		w.WriteHeader(statusCode)
		if r.Method == http.MethodHead {
			return
		}

		resp := Response{
			Status:  status,
			Details: details,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
