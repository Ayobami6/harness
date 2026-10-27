package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"harness/config"
	"harness/internal/health"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// mockToolModule implements ToolModule for testing.
type mockToolModule struct {
	toolName    string
	registerErr error
	called      bool
}

func (m *mockToolModule) RegisterTools(s *server.MCPServer) error {
	m.called = true
	if m.registerErr != nil {
		return m.registerErr
	}
	if m.toolName != "" {
		s.AddTool(mcp.NewTool(m.toolName), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText("ok"), nil
		})
	}
	return nil
}

func TestNewServer(t *testing.T) {
	t.Run("creates server without modules", func(t *testing.T) {
		cfg := config.Config{
			LogLevel: slog.LevelInfo,
			Port:     "9090",
		}

		s, err := NewServer(cfg, "test-server", "1.0.0")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil server")
		}
		if s.mcpServer == nil {
			t.Error("expected non-nil mcpServer")
		}
		if s.logger == nil {
			t.Error("expected non-nil logger")
		}
		if len(s.modules) != 0 {
			t.Errorf("expected 0 modules, got %d", len(s.modules))
		}
		if s.cfg.Port != "9090" {
			t.Errorf("expected port '9090', got %q", s.cfg.Port)
		}
	})

	t.Run("successfully registers multiple modules and tools", func(t *testing.T) {
		cfg := config.Config{
			LogLevel: slog.LevelDebug,
			Port:     "8080",
		}

		mod1 := &mockToolModule{toolName: "tool_alpha"}
		mod2 := &mockToolModule{toolName: "tool_beta"}

		s, err := NewServer(cfg, "multi-server", "2.0.0", mod1, mod2)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !mod1.called {
			t.Error("expected mod1.RegisterTools to be called")
		}
		if !mod2.called {
			t.Error("expected mod2.RegisterTools to be called")
		}
		if len(s.modules) != 2 {
			t.Fatalf("expected 2 modules, got %d", len(s.modules))
		}

		// Verify tools were registered on the underlying MCP server
		if tool := s.mcpServer.GetTool("tool_alpha"); tool == nil {
			t.Error("expected tool_alpha to be registered")
		}
		if tool := s.mcpServer.GetTool("tool_beta"); tool == nil {
			t.Error("expected tool_beta to be registered")
		}
	})

	t.Run("returns error when a module fails registration", func(t *testing.T) {
		cfg := config.Config{
			LogLevel: slog.LevelError,
			Port:     "9090",
		}

		expectedErr := errors.New("registration failed")
		failingMod := &mockToolModule{registerErr: expectedErr}

		s, err := NewServer(cfg, "failing-server", "1.0.0", failingMod)
		if err == nil {
			t.Fatal("expected NewServer to return error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected error wrapping %v, got %v", expectedErr, err)
		}
		if s != nil {
			t.Errorf("expected nil server on error, got %v", s)
		}
	})
}

func TestServer_StartHTTP_InvalidPort(t *testing.T) {
	cfg := config.Config{
		Port: "invalid-port-format",
	}

	s, err := NewServer(cfg, "http-test", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}

	err = s.StartHTTP("")
	if err == nil {
		t.Fatal("expected error when starting HTTP server with invalid port, got nil")
	}
}

func TestServer_StartStdio_ClosedPipe(t *testing.T) {
	cfg := config.Config{
		LogLevel: slog.LevelError,
	}
	s, err := NewServer(cfg, "stdio-test", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}

	// Create a pipe and immediately close the write end so stdin receives EOF
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	_ = w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		_ = r.Close()
	}()

	// ServeStdio should process EOF and return without blocking
	_ = s.StartStdio()
}

type testPinger struct {
	err error
}

func (p *testPinger) PingContext(ctx context.Context) error {
	return p.err
}

func TestServer_WithPinger(t *testing.T) {
	cfg := config.Config{Port: "8080"}
	s, err := NewServer(cfg, "pinger-test", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}

	pinger := &testPinger{}
	res := s.WithPinger(pinger)
	if res != s {
		t.Errorf("expected WithPinger to return server instance")
	}
	if s.pinger != pinger {
		t.Errorf("expected pinger to be set on server")
	}
}

func TestServer_Handler_Routes(t *testing.T) {
	cfg := config.Config{Port: "8080"}
	pinger := &testPinger{err: nil}
	s, err := NewServer(cfg, "handler-routes-test", "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error creating server: %v", err)
	}
	s.WithPinger(pinger)

	handler := s.Handler()

	// 1. Verify /healthz returns 200 OK with expected JSON
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /healthz, got %d", rec.Code)
	}

	var healthResp health.Response
	if err := json.NewDecoder(rec.Body).Decode(&healthResp); err != nil {
		t.Fatalf("failed to decode /healthz JSON response: %v", err)
	}
	if healthResp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", healthResp.Status)
	}
	if healthResp.Details["database"] != "up" {
		t.Errorf("expected database 'up', got %q", healthResp.Details["database"])
	}

	// 2. Verify /mcp route is registered and responds
	mcpReq := httptest.NewRequest(http.MethodHead, "/mcp", nil)
	mcpRec := httptest.NewRecorder()
	handler.ServeHTTP(mcpRec, mcpReq)

	if mcpRec.Code != http.StatusOK {
		t.Errorf("expected 200 OK from HEAD /mcp, got %d", mcpRec.Code)
	}
}

func TestServer_Shutdown(t *testing.T) {
	t.Run("shutdown with nil httpServer returns nil", func(t *testing.T) {
		cfg := config.Config{Port: "8080"}
		s, err := NewServer(cfg, "shutdown-nil-test", "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error creating server: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := s.Shutdown(ctx); err != nil {
			t.Errorf("expected nil error on Shutdown with nil httpServer, got %v", err)
		}
	})

	t.Run("shutdown with active httpServer shuts down cleanly", func(t *testing.T) {
		cfg := config.Config{Port: "8080"}
		s, err := NewServer(cfg, "shutdown-active-test", "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error creating server: %v", err)
		}

		customServer := &http.Server{Handler: s.Handler()}
		s.httpServer = server.NewStreamableHTTPServer(
			s.mcpServer,
			server.WithStreamableHTTPServer(customServer),
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := s.Shutdown(ctx); err != nil {
			t.Errorf("expected clean shutdown, got %v", err)
		}
	})
}
