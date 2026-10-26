package main

import (
	"context"
	"harness/config"
	"harness/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
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
	db, err := db.ConnectDB(dbctx, cfg, logger)
	if err != nil {
		logger.Error("Failed to connect to database 😡😡", "error", err)
		panic(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database connection", "error", err)
		}
	}()

}
