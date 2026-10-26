package mcp

import (
	"context"
	"errors"
	"harness/config"
	"log/slog"
	"os"
	"testing"

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

		s := NewServer(cfg, "test-server", "1.0.0")

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

		s := NewServer(cfg, "multi-server", "2.0.0", mod1, mod2)

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

	t.Run("panics when a module fails registration", func(t *testing.T) {
		cfg := config.Config{
			LogLevel: slog.LevelError,
			Port:     "9090",
		}

		expectedErr := errors.New("registration failed")
		failingMod := &mockToolModule{registerErr: expectedErr}

		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected NewServer to panic, but it did not")
			}
			err, ok := r.(error)
			if !ok {
				t.Fatalf("expected panic with error, got %T: %v", r, r)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected error %v, got %v", expectedErr, err)
			}
		}()

		NewServer(cfg, "failing-server", "1.0.0", failingMod)
	})
}

func TestServer_StartHTTP_InvalidPort(t *testing.T) {
	cfg := config.Config{
		Port: "invalid-port-format",
	}

	s := NewServer(cfg, "http-test", "1.0.0")
	err := s.StartHTTP("")
	if err == nil {
		t.Fatal("expected error when starting HTTP server with invalid port, got nil")
	}
}

func TestServer_StartStdio_ClosedPipe(t *testing.T) {
	cfg := config.Config{
		LogLevel: slog.LevelError,
	}
	s := NewServer(cfg, "stdio-test", "1.0.0")

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

