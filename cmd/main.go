package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"harness/config"
	"harness/db"
	"harness/internal/mcp"
	"harness/internal/repository"
	contexttools "harness/internal/tools/context"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		println("Failed to load configuration: " + err.Error())
		os.Exit(1)
	}
	// Create a new logger based on the loaded configuration
	logger := config.NewLogger(*cfg)

	// Use the logger to log an informational message
	logger.Info("Harness configuration loaded successfully. ✅")
	// Printing a simple message to indicate that the main function is running.
	logger.Info("Starting Harness. 🚀")
	logger.Info("Configuration Details", "DbPath", cfg.DbPath, "LogLevel", cfg.LogLevel.String(), "Port", cfg.Port, "Transport", cfg.Transport)

	println(
		`
		██╗░░██╗░█████╗░██████╗░███╗░░██╗███████╗░██████╗░██████╗
		██║░░██║██╔══██╗██╔══██╗████╗░██║██╔════╝██╔════╝██╔════╝
		███████║███████║██████╔╝██╔██╗██║█████╗░░╚█████╗░╚█████╗░
		██╔══██║██╔══██║██╔══██╗██║╚████║██╔══╝░░░╚═══██╗░╚═══██╗
		██║░░██║██║░░██║██║░░██║██║░╚███║███████╗██████╔╝██████╔╝
		╚═╝░░╚═╝╚═╝░░╚═╝╚═╝░░╚═╝╚═╝░░╚══╝╚══════╝╚═════╝░╚═════╝░
		`,
	)
	// connect db
	dbctx := context.Background()
	conn, err := db.ConnectDB(dbctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("Failed to close database connection", "error", err)
		}
	}()

	repo := repository.New(conn)
	contextMod := contexttools.New(repo)

	srv, err := mcp.NewServer(*cfg, "harness", "1.0.0", contextMod)
	if err != nil {
		logger.Error("Failed to initialize MCP server", "error", err)
		os.Exit(1)
	}
	srv.WithPinger(conn)

	if cfg.Transport == "stdio" {
		if err := srv.StartStdio(); err != nil {
			logger.Error("MCP server failed over stdio", "error", err)
			os.Exit(1)
		}
	} else {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			<-ctx.Done()
			logger.Info("Shutdown signal received, shutting down gracefully...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				logger.Error("Failed to gracefully shutdown server", "error", err)
			}
		}()

		if err := srv.StartHTTP(cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("MCP server failed over http", "error", err)
			os.Exit(1)
		}
	}
}
