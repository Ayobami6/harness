package mcp

import (
	"context"
	"errors"
	"fmt"
	"harness/config"
	"harness/internal/health"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// ToolModule interface defines the methods that a tool module should implement to be compatible with the MCP framework.
type ToolModule interface {
	RegisterTools(s *server.MCPServer) error
}

type Server struct {
	// Add fields as necessary for the server configuration
	mcpServer  *server.MCPServer
	httpServer *server.StreamableHTTPServer
	modules    []ToolModule
	cfg        config.Config
	pinger     health.Pinger
	logger     *slog.Logger
	mu         sync.Mutex
}

// NewServer initializes a new MCP server and registers the provided tool modules.
// If any module fails to register, it returns an error instead of panicking.
func NewServer(cfg config.Config, name, version string, modules ...ToolModule) (*Server, error) {
	s := server.NewMCPServer(name, version, server.WithToolCapabilities(true))
	logger := config.NewLogger(cfg)
	for _, module := range modules {
		if err := module.RegisterTools(s); err != nil {
			logger.Error("Failed to register tools for module", "error", err)
			return nil, fmt.Errorf("failed to register tools for module: %w", err)
		}
	}
	return &Server{
		mcpServer: s,
		modules:   modules,
		cfg:       cfg,
		logger:    logger,
	}, nil
}

// WithPinger sets the database pinger for health checking.
func (s *Server) WithPinger(pinger health.Pinger) *Server {
	s.pinger = pinger
	return s
}

// Handler returns an http.Handler with all registered HTTP routes (/healthz, /mcp).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewHandler(s.pinger))

	httpServer := server.NewStreamableHTTPServer(s.mcpServer)
	mux.Handle("/mcp", httpServer)
	return mux
}

// StartStdio starts the MCP server and serves it over standard input/output (stdio).
func (s *Server) StartStdio() error {
	s.logger.Info("Starting MCP Server over stdio")
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("failed to serve mcp over stdio: %w", err)
	}
	return nil
}

// StartHTTP starts the MCP server and serves it over HTTP on the specified address or configured port.
func (s *Server) StartHTTP(address string) error {
	port := s.cfg.Port
	if address != "" {
		port = address
	}
	addr := port
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	mux := http.NewServeMux()
	mux.Handle("/healthz", health.NewHandler(s.pinger))

	customServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	httpServer := server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithStreamableHTTPServer(customServer),
	)
	mux.Handle("/mcp", httpServer)

	s.mu.Lock()
	s.httpServer = httpServer
	s.mu.Unlock()

	s.logger.Info("🚀 Starting MCP Server over HTTP 🚀", "address", addr, "endpoint", "/mcp", "healthz", "/healthz")
	if err := httpServer.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start mcp http server on %s: %w", addr, err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server and its active MCP sessions.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	hs := s.httpServer
	s.mu.Unlock()

	if hs != nil {
		if err := hs.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to gracefully shutdown mcp server: %w", err)
		}
	}
	return nil
}
