package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/api"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/config"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/knowledge"
	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/openai"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	slog.Info("Starting claude-agent-proxy-svc")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found or error loading it", "error", err)
	}

	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		slog.Error("Failed to process config", "error", err)
		os.Exit(1)
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err == nil {
		opts := &slog.HandlerOptions{Level: level}
		logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
		slog.SetDefault(logger)
	}

	slog.Info("Starting Claude Agent Proxy Service",
		"port", cfg.Port,
		"claude_model", cfg.ClaudeModel,
		"knowledge_enabled", cfg.KnowledgeEnabled,
		"knowledge_path", cfg.KnowledgeBasePath,
	)

	// Initialize knowledge management system if enabled
	var knowledgeRetriever *knowledge.Retriever
	var storageManager *knowledge.StorageManager
	if cfg.KnowledgeEnabled {
		// Create knowledge storage manager
		var err error
		storageManager, err = knowledge.NewStorageManager(cfg.KnowledgeBasePath, logger)
		if err != nil {
			slog.Error("Failed to initialize knowledge storage manager", "error", err)
			os.Exit(1)
		}

		// Create knowledge retriever
		knowledgeRetriever = knowledge.NewRetriever(storageManager, logger)

		// Log successful initialization
		slog.Info("Knowledge management system initialized", "base_path", cfg.KnowledgeBasePath)
	}

	claudeClient := openai.NewClient(cfg.ClaudeAPIKey, cfg.ClaudeModel, logger)
	handler := api.NewHandler(claudeClient, logger, knowledgeRetriever)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	
	// Register knowledge management routes if enabled
	if cfg.KnowledgeEnabled && storageManager != nil {
		knowledgeHandler := knowledge.NewHandler(storageManager, logger)
		knowledgeHandler.RegisterRoutes(mux)
		slog.Info("Knowledge management API routes registered")
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	go func() {
		slog.Info("Starting HTTP server", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	slog.Info("Received signal, shutting down", "signal", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
	}

	slog.Info("Service shutdown complete")
}
