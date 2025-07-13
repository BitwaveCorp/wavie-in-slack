package models

import (
	"time"
)

// DocumentChunk represents a chunk of a document
type DocumentChunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Content    string    `json:"content"`
	ChunkIndex int       `json:"chunk_index"`
	CreatedAt  time.Time `json:"created_at"`
}

// Embedding represents a vector embedding
type Embedding struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	ChunkIndex int       `json:"chunk_index"`
	Vector     []float32 `json:"vector"`
	CreatedAt  time.Time `json:"created_at"`
}

// DocumentUploadRequest represents a document upload request
type DocumentUploadRequest struct {
	DocumentID string `json:"document_id"`
	FilePath   string `json:"file_path"`
}

// DocumentUploadResponse represents a document upload response
type DocumentUploadResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	DocumentID string `json:"document_id"`
	ChunkCount int    `json:"chunk_count"`
}

// QuestionRequest represents a question request
type QuestionRequest struct {
	Question string `json:"question"`
}

// QuestionResponse represents a question response
type QuestionResponse struct {
	Answer       string   `json:"answer"`
	SourceChunks []string `json:"source_chunks,omitempty"`
}

// DeleteDocumentRequest represents a document deletion request
type DeleteDocumentRequest struct {
	DocumentID string `json:"document_id"`
}

// DeleteDocumentResponse represents a document deletion response
type DeleteDocumentResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	DocumentID string `json:"document_id"`
}

// HealthCheckResponse represents a health check response
type HealthCheckResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}
