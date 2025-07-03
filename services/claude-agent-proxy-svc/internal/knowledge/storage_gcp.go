package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCPStorageManager implements StorageBackend using Google Cloud Storage
type GCPStorageManager struct {
	bucketName      string
	registryKey     string
	client          *storage.Client
	registry        KnowledgeRegistry
	mutex           sync.RWMutex
	logger          *slog.Logger
	localTempDir    string // For temporary file operations
}

// NewGCPStorageManager creates a new GCP storage manager
func NewGCPStorageManager(ctx context.Context, bucketName, registryKey string, logger *slog.Logger, localTempDir string, opts ...option.ClientOption) (*GCPStorageManager, error) {
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP storage client: %w", err)
	}

	sm := &GCPStorageManager{
		bucketName:   bucketName,
		registryKey:  registryKey,
		client:       client,
		registry: KnowledgeRegistry{
			Agents:        []Agent{},
			KnowledgeFiles: []KnowledgeFile{},
		},
		logger:       logger,
		localTempDir: localTempDir,
	}

	// Check if bucket exists, create if it doesn't
	if err := sm.ensureBucketExists(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	// Load existing registry if it exists
	if err := sm.loadRegistry(ctx); err != nil {
		// If registry doesn't exist, create default agent
		if err == storage.ErrObjectNotExist {
			defaultAgent := Agent{
				ID:          "wavie-bot",
				Name:        "Wavie Bot",
				Description: "Default Wavie Bot agent for Slack",
				TenantID:    "bitwave",
				CreatedAt:   time.Now(),
			}
			sm.registry.Agents = append(sm.registry.Agents, defaultAgent)
			
			if err := sm.saveRegistry(ctx); err != nil {
				return nil, fmt.Errorf("failed to save initial registry: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load registry: %w", err)
		}
	}

	return sm, nil
}

// ensureBucketExists checks if the bucket exists and creates it if it doesn't
func (sm *GCPStorageManager) ensureBucketExists(ctx context.Context) error {
	bucket := sm.client.Bucket(sm.bucketName)
	_, err := bucket.Attrs(ctx)
	if err == storage.ErrBucketNotExist {
		if err := bucket.Create(ctx, "", nil); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}
	return nil
}

// loadRegistry loads the registry from GCS
func (sm *GCPStorageManager) loadRegistry(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	reader, err := sm.client.Bucket(sm.bucketName).Object(sm.registryKey).NewReader(ctx)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := json.NewDecoder(reader).Decode(&sm.registry); err != nil {
		return fmt.Errorf("failed to parse registry: %w", err)
	}

	return nil
}

// saveRegistry saves the registry to GCS
func (sm *GCPStorageManager) saveRegistry(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	writer := sm.client.Bucket(sm.bucketName).Object(sm.registryKey).NewWriter(ctx)
	defer writer.Close()

	if err := json.NewEncoder(writer).Encode(sm.registry); err != nil {
		return fmt.Errorf("failed to encode registry: %w", err)
	}

	return nil
}

// StoreKnowledgeFile stores a knowledge file in GCS
func (sm *GCPStorageManager) StoreKnowledgeFile(name, description string, agentIDs []string, fileData io.Reader, contentType string) (*KnowledgeFile, error) {
	ctx := context.Background()
	
	// Generate unique ID for the file
	fileID := uuid.New().String()
	
	// Path in GCS for this file
	filePath := fmt.Sprintf("files/%s", fileID)
	zipPath := fmt.Sprintf("%s/content.zip", filePath)
	
	// Upload the zip file to GCS
	writer := sm.client.Bucket(sm.bucketName).Object(zipPath).NewWriter(ctx)
	writer.ContentType = contentType
	
	size, err := io.Copy(writer, fileData)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to upload file to GCS: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize GCS upload: %w", err)
	}
	
	// Create knowledge file record
	knowledgeFile := KnowledgeFile{
		ID:          fileID,
		Name:        name,
		Description: description,
		FilePath:    filePath, // Store GCS path instead of local path
		AgentIDs:    agentIDs,
		UploadedAt:  time.Now(),
		FileSize:    size,
		ContentType: contentType,
	}
	
	// Update registry
	sm.mutex.Lock()
	sm.registry.KnowledgeFiles = append(sm.registry.KnowledgeFiles, knowledgeFile)
	sm.mutex.Unlock()
	
	// Save registry
	if err := sm.saveRegistry(ctx); err != nil {
		return nil, fmt.Errorf("failed to update registry: %w", err)
	}
	
	return &knowledgeFile, nil
}

// GetKnowledgeFilesForAgent returns all knowledge files associated with an agent
func (sm *GCPStorageManager) GetKnowledgeFilesForAgent(agentID string) []KnowledgeFile {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	var files []KnowledgeFile
	for _, file := range sm.registry.KnowledgeFiles {
		for _, id := range file.AgentIDs {
			if id == agentID {
				files = append(files, file)
				break
			}
		}
	}
	
	return files
}

// GetAllKnowledgeFiles returns all knowledge files
func (sm *GCPStorageManager) GetAllKnowledgeFiles() []KnowledgeFile {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	files := make([]KnowledgeFile, len(sm.registry.KnowledgeFiles))
	copy(files, sm.registry.KnowledgeFiles)
	
	return files
}

// GetKnowledgeFile returns a knowledge file by ID
func (sm *GCPStorageManager) GetKnowledgeFile(id string) (*KnowledgeFile, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	for _, file := range sm.registry.KnowledgeFiles {
		if file.ID == id {
			return &file, nil
		}
	}
	return nil, fmt.Errorf("knowledge file not found: %s", id)
}

// DeleteKnowledgeFile deletes a knowledge file by ID
func (sm *GCPStorageManager) DeleteKnowledgeFile(id string) error {
	ctx := context.Background()
	
	// Find the file in the registry
	var fileIndex int = -1
	var filePath string
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	for i, file := range sm.registry.KnowledgeFiles {
		if file.ID == id {
			fileIndex = i
			filePath = file.FilePath
			break
		}
	}

	if fileIndex == -1 {
		return fmt.Errorf("knowledge file not found: %s", id)
	}

	// Remove the file from the registry
	sm.registry.KnowledgeFiles = append(
		sm.registry.KnowledgeFiles[:fileIndex],
		sm.registry.KnowledgeFiles[fileIndex+1:]...,
	)

	// Save the updated registry
	err := sm.saveRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// Delete all objects with this prefix from GCS
	if filePath != "" {
		bucket := sm.client.Bucket(sm.bucketName)
		it := bucket.Objects(ctx, &storage.Query{Prefix: filePath})
		for {
			attrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return fmt.Errorf("error listing objects to delete: %w", err)
			}
			if err := bucket.Object(attrs.Name).Delete(ctx); err != nil {
				return fmt.Errorf("failed to delete object %s: %w", attrs.Name, err)
			}
		}
	}

	return nil
}

// GetAllAgents returns all agents
func (sm *GCPStorageManager) GetAllAgents() []Agent {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	agents := make([]Agent, len(sm.registry.Agents))
	copy(agents, sm.registry.Agents)
	
	// Remove sensitive information
	for i := range agents {
		agents[i].ApiKey = ""
	}
	
	return agents
}

// GetAgent returns an agent by ID
func (sm *GCPStorageManager) GetAgent(agentID string) *Agent {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	for _, agent := range sm.registry.Agents {
		if agent.ID == agentID {
			// Return a copy to avoid race conditions
			agentCopy := agent
			agentCopy.ApiKey = ""
			return &agentCopy
		}
	}
	
	return nil
}

// CreateAgent creates a new agent
func (sm *GCPStorageManager) CreateAgent(id, name, description, tenantID string) (*Agent, error) {
	ctx := context.Background()
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	// Check if agent already exists
	for _, agent := range sm.registry.Agents {
		if agent.ID == id {
			return nil, fmt.Errorf("agent with ID %s already exists", id)
		}
	}
	
	// Create new agent
	agent := Agent{
		ID:          id,
		Name:        name,
		Description: description,
		TenantID:    tenantID,
		ApiKey:      uuid.New().String(), // Generate API key
		CreatedAt:   time.Now(),
	}
	
	sm.registry.Agents = append(sm.registry.Agents, agent)
	
	// Save registry
	if err := sm.saveRegistry(ctx); err != nil {
		return nil, fmt.Errorf("failed to save registry: %w", err)
	}
	
	// Return a copy without API key
	agentCopy := agent
	agentCopy.ApiKey = ""
	
	return &agentCopy, nil
}

// Close closes the GCP client
func (sm *GCPStorageManager) Close() error {
	return sm.client.Close()
}
