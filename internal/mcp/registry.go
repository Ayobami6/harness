package mcp

import (
	"fmt"
	"harness/config"
	"log/slog"

	"github.com/mark3labs/mcp-go/server"
)

// ToolModule interface defines the methods that a tool module should implement to be compatible with the MCP framework.
type ToolModule interface {
	RegisterTools(s *server.MCPServer) error
}

type Server struct {
	// Add fields as necessary for the server configuration
	mcpServer *server.MCPServer
	modules   []ToolModule
	cfg       config.Config
	logger    *slog.Logger
}

// NewServer initializes a new MCP server and registers the provided tool modules.
// If any module fails to register, it returns an error instead of panicking.
func NewServer(cfg config.Config, name, version string, modules ...ToolModule) (*Server, error) {
	s := server.NewMCPServer(name, version, server.WithToolCapabilities(true))
	logger := config.NewLogger(cfg)
	for _, module := range modules {
		if err := module.RegisterTools(s); err != nil {
			logger.Error("Failed to register tools for module 😡⚠️", "error", err)
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

// StartStdio starts the MCP server and serves it over standard input/output (stdio).
func (s *Server) StartStdio() error {
	s.logger.Info("🚀 Starting MCP Server over stdio 🚀")
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
	httpServer := server.NewStreamableHTTPServer(s.mcpServer)
	if err := httpServer.Start(port); err != nil {
		return fmt.Errorf("failed to start mcp http server on port %s: %w", port, err)
	}
	return nil
}
