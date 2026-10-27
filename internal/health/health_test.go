package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) PingContext(ctx context.Context) error {
	return m.err
}

func TestNewHandler(t *testing.T) {
	t.Run("GET with healthy database returns 200 and up details", func(t *testing.T) {
		pinger := &mockPinger{err: nil}
		handler := NewHandler(pinger)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", contentType)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode json response: %v", err)
		}

		if resp.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", resp.Status)
		}
		if resp.Details["database"] != "up" {
			t.Errorf("expected database 'up', got %q", resp.Details["database"])
		}
	})

	t.Run("HEAD with healthy database returns 200 with empty body", func(t *testing.T) {
		pinger := &mockPinger{err: nil}
		handler := NewHandler(pinger)

		req := httptest.NewRequest(http.MethodHead, "/healthz", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", contentType)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected empty body for HEAD request, got %q", rec.Body.String())
		}
	})

	t.Run("GET with nil database returns 200 and ok status", func(t *testing.T) {
		handler := NewHandler(nil)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode json response: %v", err)
		}

		if resp.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", resp.Status)
		}
		if len(resp.Details) != 0 {
			t.Errorf("expected empty details with nil pinger, got %v", resp.Details)
		}
	})

	t.Run("GET with failing database returns 503 and down details", func(t *testing.T) {
		pinger := &mockPinger{err: errors.New("connection refused")}
		handler := NewHandler(pinger)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", rec.Code)
		}

		var resp Response
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode json response: %v", err)
		}

		if resp.Status != "unavailable" {
			t.Errorf("expected status 'unavailable', got %q", resp.Status)
		}
		if resp.Details["database"] != "down" {
			t.Errorf("expected database 'down', got %q", resp.Details["database"])
		}
	})

	t.Run("HEAD with failing database returns 503 with empty body", func(t *testing.T) {
		pinger := &mockPinger{err: errors.New("timeout")}
		handler := NewHandler(pinger)

		req := httptest.NewRequest(http.MethodHead, "/healthz", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected empty body for HEAD request, got %q", rec.Body.String())
		}
	})

	t.Run("Disallowed HTTP methods return 405 Method Not Allowed", func(t *testing.T) {
		methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		handler := NewHandler(nil)

		for _, method := range methods {
			t.Run(method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/healthz", nil)
				rec := httptest.NewRecorder()

				handler(rec, req)

				if rec.Code != http.StatusMethodNotAllowed {
					t.Errorf("expected status 405 for %s, got %d", method, rec.Code)
				}
				if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
					t.Errorf("expected Allow header 'GET, HEAD', got %q", allow)
				}
			})
		}
	})
}
