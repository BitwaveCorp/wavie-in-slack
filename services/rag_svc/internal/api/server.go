package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/bitwavecorp/wavie-in-slack/services/rag_svc/internal/config"
	"github.com/bitwavecorp/wavie-in-slack/services/rag_svc/internal/embedding"
	"github.com/bitwavecorp/wavie-in-slack/services/rag_svc/internal/models"
	"github.com/bitwavecorp/wavie-in-slack/services/rag_svc/internal/processor"
	"github.com/bitwavecorp/wavie-in-slack/services/rag_svc/internal/storage"
)

// Server represents the API server
type Server struct {
	router           *mux.Router
	config           *config.Config
	docProcessor     *processor.DocumentProcessor
	embeddingService *embedding.EmbeddingService
	vectorStore      *storage.VectorStore
	version          string
}

// NewServer creates a new API server
func NewServer(ctx context.Context, cfg *config.Config) (*Server, error) {
	// Initialize document processor
	docProcessor, err := processor.NewDocumentProcessor(ctx, cfg.GCP.StorageBucket, 1000, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to create document processor: %v", err)
	}

	// Initialize embedding service
	embeddingService := embedding.NewEmbeddingService(cfg.OpenAI.APIKey, cfg.OpenAI.Model)

	// Initialize vector store
	vectorStore, err := storage.NewVectorStore(
		ctx,
		cfg.GCP.ProjectID,
		cfg.GCP.Location,
		cfg.Vertex.IndexID,
		cfg.Vertex.IndexEndpointID,
		cfg.Vertex.DeployedIndexID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %v", err)
	}

	// Create server
	server := &Server{
		router:           mux.NewRouter(),
		config:           cfg,
		docProcessor:     docProcessor,
		embeddingService: embeddingService,
		vectorStore:      vectorStore,
		version:          "1.0.0",
	}

	// Set up routes
	server.setupRoutes()

	return server, nil
}

// Close closes the server and its resources
func (s *Server) Close() error {
	var errs []error

	if err := s.vectorStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close vector store: %v", err))
	}

	if err := s.docProcessor.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close document processor: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing server: %v", errs)
	}

	return nil
}

// setupRoutes sets up the API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.handleHealthCheck).Methods("GET")

	// Document upload
	s.router.HandleFunc("/api/documents", s.handleUploadDocument).Methods("POST")

	// Document deletion
	s.router.HandleFunc("/api/documents/{documentId}", s.handleDeleteDocument).Methods("DELETE")

	// Document deletion by prefix
	s.router.HandleFunc("/api/documents/prefix/{prefix}", s.handleDeleteDocumentByPrefix).Methods("DELETE")

	// Question answering
	s.router.HandleFunc("/api/ask", s.handleAskQuestion).Methods("POST")
}

// Start starts the server
func (s *Server) Start() error {
	log.Printf("Starting RAG service on port %s", s.config.Server.Port)
	return http.ListenAndServe(":"+s.config.Server.Port, s.router)
}

// handleUploadDocument handles document uploads
func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req models.DocumentUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.DocumentID == "" || req.FilePath == "" {
		http.Error(w, "Missing document ID or file path", http.StatusBadRequest)
		return
	}

	// Get document content from GCP Storage
	content, err := s.docProcessor.GetDocument(ctx, req.FilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get document: %v", err), http.StatusInternalServerError)
		return
	}

	// Process document based on file type
	var chunks []string
	if strings.HasSuffix(req.FilePath, ".md") {
		chunks = s.docProcessor.ProcessMarkdown(content)
	} else {
		chunks = s.docProcessor.ChunkDocument(content)
	}

	// Generate embeddings
	embeddings, err := s.embeddingService.BatchGenerateEmbeddings(ctx, chunks)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate embeddings: %v", err), http.StatusInternalServerError)
		return
	}

	// Store embeddings
	err = s.vectorStore.StoreEmbeddings(ctx, req.DocumentID, chunks, embeddings)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to store embeddings: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate token count and cost
	totalTokens := 0
	for _, chunk := range chunks {
		totalTokens += s.docProcessor.EstimateTokenCount(chunk)
	}
	cost := s.embeddingService.EstimateEmbeddingCost(totalTokens)

	// Return success
	response := models.DocumentUploadResponse{
		Status:     "success",
		Message:    fmt.Sprintf("Document processed successfully. Estimated cost: $%.6f", cost),
		DocumentID: req.DocumentID,
		ChunkCount: len(chunks),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDeleteDocument handles document deletion
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get document ID from URL
	vars := mux.Vars(r)
	documentID := vars["documentId"]

	if documentID == "" {
		http.Error(w, "Missing document ID", http.StatusBadRequest)
		return
	}

	// Delete document embeddings
	err := s.vectorStore.DeleteDocumentEmbeddings(ctx, documentID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete document embeddings: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success
	response := models.DeleteDocumentResponse{
		Status:     "success",
		Message:    "Document embeddings deleted successfully",
		DocumentID: documentID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleDeleteDocumentByPrefix handles deletion of all documents with IDs starting with the given prefix
func (s *Server) handleDeleteDocumentByPrefix(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get prefix from URL
	vars := mux.Vars(r)
	prefix := vars["prefix"]

	if prefix == "" {
		http.Error(w, "Missing prefix", http.StatusBadRequest)
		return
	}

	// Log the request
	log.Printf("Deleting all document embeddings with prefix: %s", prefix)

	// Delete document embeddings by prefix
	err := s.vectorStore.DeleteDocumentEmbeddingsByPrefix(ctx, prefix)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete document embeddings by prefix: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success
	response := models.DeleteDocumentResponse{
		Status:     "success",
		Message:    fmt.Sprintf("Document embeddings with prefix '%s' deleted successfully", prefix),
		DocumentID: prefix, // Using the prefix as the document ID in the response
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleAskQuestion handles question answering
func (s *Server) handleAskQuestion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request
	var req models.QuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Question == "" {
		http.Error(w, "Missing question", http.StatusBadRequest)
		return
	}

	// Generate embedding for question
	queryEmbedding, err := s.embeddingService.GenerateEmbedding(ctx, req.Question)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate query embedding: %v", err), http.StatusInternalServerError)
		return
	}

	// Find similar chunks
	similarChunks, err := s.vectorStore.FindSimilarChunks(ctx, queryEmbedding, 3)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find similar chunks: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the chunks (in a real implementation, you would send these to Claude)
	response := models.QuestionResponse{
		Answer:       "This is a placeholder. In production, these chunks would be sent to Claude for processing.",
		SourceChunks: similarChunks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleHealthCheck handles health check requests
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	response := models.HealthCheckResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   s.version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
