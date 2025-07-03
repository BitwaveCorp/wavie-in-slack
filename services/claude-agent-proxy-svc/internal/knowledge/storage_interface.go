package knowledge

import (
	"io"
)

// StorageBackend defines the interface for storage backends
type StorageBackend interface {
	// StoreKnowledgeFile stores a knowledge file and returns its metadata and extraction result
	StoreKnowledgeFile(name, description string, agentIDs []string, fileData io.Reader, contentType string) (*KnowledgeFile, *ExtractionResult, error)
	
	// GetKnowledgeFile retrieves a knowledge file by ID
	GetKnowledgeFile(id string) (*KnowledgeFile, error)
	
	// GetKnowledgeFilesForAgent returns all knowledge files associated with an agent
	GetKnowledgeFilesForAgent(agentID string) []KnowledgeFile
	
	// GetAllKnowledgeFiles returns all knowledge files
	GetAllKnowledgeFiles() []KnowledgeFile
	
	// DeleteKnowledgeFile deletes a knowledge file by ID
	DeleteKnowledgeFile(id string) error
	
	// GetAllAgents returns all agents
	GetAllAgents() []Agent
	
	// GetAgent returns an agent by ID
	GetAgent(agentID string) *Agent
	
	// CreateAgent creates a new agent
	CreateAgent(id, name, description, tenantID string) (*Agent, error)
}
