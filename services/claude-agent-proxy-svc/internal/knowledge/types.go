package knowledge

import (
	"time"
)

// KnowledgeFile represents a knowledge file in the system
type KnowledgeFile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	FilePath    string    `json:"file_path"`
	AgentIDs    []string  `json:"agent_ids"`
	UploadedAt  time.Time `json:"uploaded_at"`
	FileSize    int64     `json:"file_size"`
	ContentType string    `json:"content_type"`
}

// Agent represents an AI agent in the system
type Agent struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TenantID    string   `json:"tenant_id"`
	ApiKey      string   `json:"api_key,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// KnowledgeRegistry stores the mapping between agents and knowledge files
type KnowledgeRegistry struct {
	Agents        []Agent        `json:"agents"`
	KnowledgeFiles []KnowledgeFile `json:"knowledge_files"`
}

// UploadRequest represents a request to upload a knowledge file
type UploadRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AgentIDs    []string `json:"agent_ids"`
}

// UploadResponse represents a response to an upload request
type UploadResponse struct {
	Success bool   `json:"success"`
	FileID  string `json:"file_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ListFilesResponse represents a response to a list files request
type ListFilesResponse struct {
	Files []KnowledgeFile `json:"files"`
}

// ListAgentsResponse represents a response to a list agents request
type ListAgentsResponse struct {
	Agents  []Agent `json:"agents"`
}

// Agent request/response types moved to handler.go
