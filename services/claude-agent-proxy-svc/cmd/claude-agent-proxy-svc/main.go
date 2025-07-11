package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	// Set up panic recovery to ensure we log any fatal errors
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "FATAL PANIC: %v\n", r)
			// Try to log to stdout as well in case stderr isn't captured
			fmt.Printf("FATAL PANIC: %v\n", r)
			os.Exit(1)
		}
	}()
	
	// Log startup immediately to stderr and stdout to ensure it's captured
	fmt.Fprintf(os.Stderr, "Starting claude-agent-proxy-svc\n")
	fmt.Printf("Starting claude-agent-proxy-svc\n")
	
	// Set up basic logging first
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	
	slog.Info("Starting claude-agent-proxy-svc")
	
	// Log all environment variables at startup
	slog.Info("Environment variables at startup:")
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			
			// Mask sensitive values
			if strings.Contains(strings.ToLower(key), "key") || 
			   strings.Contains(strings.ToLower(key), "secret") || 
			   strings.Contains(strings.ToLower(key), "password") || 
			   strings.Contains(strings.ToLower(key), "token") {
				if len(value) > 8 {
					value = value[:4] + "..." + value[len(value)-4:]
				} else {
					value = "***masked***"
				}
			}
			
			slog.Info("ENV", "key", key, "value", value)
		}
	}

	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found or error loading it", "error", err)
	}

	slog.Info("Starting config processing")
	var cfg config.Config
	
	// Log the required environment variables before processing
	claudeAPIKey := os.Getenv("CLAUDE_API_KEY")
	if claudeAPIKey == "" {
		slog.Error("CLAUDE_API_KEY environment variable is not set")
	} else {
		prefix := ""
		if len(claudeAPIKey) >= 4 {
			prefix = claudeAPIKey[:4]
		} else {
			prefix = claudeAPIKey
		}
		slog.Info("CLAUDE_API_KEY is set", "length", len(claudeAPIKey), "prefix", prefix)
	}
	
	storageType := os.Getenv("STORAGE_TYPE")
	slog.Info("Storage configuration", "STORAGE_TYPE", storageType)
	
	if storageType == "gcp" {
		gcpBucket := os.Getenv("GCP_STORAGE_BUCKET")
		gcpProject := os.Getenv("GCP_PROJECT_ID")
		slog.Info("GCP storage configuration", 
			"GCP_STORAGE_BUCKET", gcpBucket, 
			"GCP_PROJECT_ID", gcpProject, 
			"GCP_KEY_FILE_SET", os.Getenv("GCP_KEY_FILE") != "")
	}
	
	// Process config
	slog.Info("Processing configuration with envconfig")
	if err := envconfig.Process("", &cfg); err != nil {
		slog.Error("Failed to process config", "error", err)
		// Log more details about the error
		slog.Error("Config processing error details", "error_type", fmt.Sprintf("%T", err), "error_string", err.Error())
		os.Exit(1)
	}
	slog.Info("Configuration processed successfully")

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
	var storageBackend knowledge.StorageBackend
	if cfg.KnowledgeEnabled {
		// Create storage configuration from app config
		storageConfig := knowledge.StorageConfig{
			Type:       knowledge.StorageType(cfg.Knowledge.StorageType),
			LocalPath:  cfg.KnowledgeBasePath,
			GCPBucket:  cfg.Knowledge.GCPBucket,
			GCPProject: cfg.Knowledge.GCPProject,
			GCPKeyFile: cfg.Knowledge.GCPKeyFile,
		}
		
		// Log storage configuration details before initialization
		slog.Info("Initializing storage backend with config", 
			"storage_type", storageConfig.Type, 
			"local_dir", storageConfig.LocalPath,
			"gcp_bucket", storageConfig.GCPBucket,
			"gcp_project", storageConfig.GCPProject,
			"gcp_key_file_set", storageConfig.GCPKeyFile != "")
	
		// Try to initialize storage backend with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
	
		slog.Info("Calling NewStorageBackend")
		var err error
		storageBackend, err = knowledge.NewStorageBackend(ctx, storageConfig, logger)
		if err != nil {
			slog.Error("Failed to initialize knowledge storage backend", 
				"error", err, 
				"error_type", fmt.Sprintf("%T", err),
				"storage_type", storageConfig.Type)
			
			// Try to log more details about GCP errors
			if storageConfig.Type == "gcp" {
				slog.Error("GCP storage backend initialization failed", 
					"gcp_bucket", storageConfig.GCPBucket,
					"gcp_project", storageConfig.GCPProject)
			}
		
			os.Exit(1)
		}
		slog.Info("Storage backend initialized successfully", "storage_type", storageConfig.Type)

		// Create knowledge retriever
		knowledgeRetriever = knowledge.NewRetriever(storageBackend, logger)

		// Log successful initialization
		slog.Info("Knowledge management system initialized", 
			"base_path", cfg.KnowledgeBasePath,
			"storage_type", storageConfig.Type)
	}

	claudeClient := openai.NewClient(cfg.ClaudeAPIKey, cfg.ClaudeModel, logger)
	handler := api.NewHandler(claudeClient, logger, knowledgeRetriever)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	
	// Register knowledge management routes if enabled
	if cfg.KnowledgeEnabled && storageBackend != nil {
		knowledgeHandler := knowledge.NewHandler(storageBackend, logger)
		knowledgeHandler.RegisterRoutes(mux)
		slog.Info("Knowledge management API routes registered")
	}

	// Validate and log port configuration
	portStr := os.Getenv("PORT")
	slog.Info("Port configuration", "config_port", cfg.Port, "env_PORT", portStr)
	
	// In Cloud Run, the PORT environment variable must be respected
	serverPort := cfg.Port
	if portStr != "" {
		if parsedPort, err := strconv.Atoi(portStr); err == nil {
			if parsedPort != cfg.Port {
				slog.Warn("PORT environment variable differs from config port, using PORT env var", 
					"env_port", parsedPort, "config_port", cfg.Port)
				serverPort = parsedPort
			}
		} else {
			slog.Error("Failed to parse PORT environment variable", "PORT", portStr, "error", err)
		}
	}
	
	// Start HTTP server
	serverAddr := fmt.Sprintf(":%d", serverPort)
	slog.Info("Starting HTTP server", "address", serverAddr)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	go func() {
		slog.Info("Starting HTTP server", "port", serverPort)
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
