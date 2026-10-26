package mcp

import (
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

func NewServer(cfg config.Config, name, version string, modules ...ToolModule) *Server {
	s := server.NewMCPServer(name, version, server.WithToolCapabilities(true))
	// let's register the modules here
	logger := config.NewLogger(cfg)
	for _, module := range modules {
		if err := module.RegisterTools(s); err != nil {
			logger.Error("Failed to register tools for module 😡⚠️", "error", err)
			panic(err)
		}
	}
	return &Server{
		mcpServer: s,
		modules:   modules,
		cfg:       cfg,
		logger:    logger,
	}
}

// StartStdio starts the MCP server and serves it over standard input/output (stdio).
// It registers all provided tool modules before starting the server. If any module fails to register, it logs the error and returns it.
func (s *Server) StartStdio() error {
	s.logger.Info("🚀 Starting MCP Server over stdio 🚀")
	// Start the MCP server and serve over stdio
	return server.ServeStdio(s.mcpServer)
}

// StartHTTP starts the MCP server and serves it over HTTP on the specified address.
// It registers all provided tool modules before starting the server. If any module fails to register, it logs the error and returns it.
func (s *Server) StartHTTP(address string) error {

	// Start the MCP server and serve over HTTP
	httpServer := server.NewStreamableHTTPServer(s.mcpServer)
	return httpServer.Start(s.cfg.Port)
}
