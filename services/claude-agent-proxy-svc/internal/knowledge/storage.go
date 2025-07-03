package knowledge

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StorageManager handles knowledge file storage and retrieval using the local filesystem
// It implements the StorageBackend interface
type StorageManager struct {
	basePath    string
	registryPath string
	registry    KnowledgeRegistry
	mutex       sync.RWMutex
	logger      *slog.Logger
}

// NewStorageManager creates a new storage manager
func NewStorageManager(basePath string, logger *slog.Logger) (*StorageManager, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	// Create knowledge files directory
	filesPath := filepath.Join(basePath, "files")
	if err := os.MkdirAll(filesPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create files directory: %w", err)
	}

	registryPath := filepath.Join(basePath, "registry.json")
	
	sm := &StorageManager{
		basePath:    basePath,
		registryPath: registryPath,
		registry:    KnowledgeRegistry{
			Agents:        []Agent{},
			KnowledgeFiles: []KnowledgeFile{},
		},
		logger:      logger,
	}

	// Load existing registry if it exists
	if _, err := os.Stat(registryPath); err == nil {
		if err := sm.loadRegistry(); err != nil {
			return nil, fmt.Errorf("failed to load registry: %w", err)
		}
	} else {
		// Create default agent if registry doesn't exist
		defaultAgent := Agent{
			ID:          "wavie-bot",
			Name:        "Wavie Bot",
			Description: "Default Wavie Bot agent for Slack",
			TenantID:    "bitwave",
			CreatedAt:   time.Now(),
		}
		sm.registry.Agents = append(sm.registry.Agents, defaultAgent)
		
		if err := sm.saveRegistry(); err != nil {
			return nil, fmt.Errorf("failed to save initial registry: %w", err)
		}
	}

	return sm, nil
}

// loadRegistry loads the registry from disk
func (sm *StorageManager) loadRegistry() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	data, err := os.ReadFile(sm.registryPath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	if err := json.Unmarshal(data, &sm.registry); err != nil {
		return fmt.Errorf("failed to parse registry: %w", err)
	}

	return nil
}

// saveRegistry saves the registry to disk
func (sm *StorageManager) saveRegistry() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	data, err := json.MarshalIndent(sm.registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(sm.registryPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	return nil
}

// StoreKnowledgeFile stores a knowledge file
func (sm *StorageManager) StoreKnowledgeFile(name, description string, agentIDs []string, fileData io.Reader, contentType string) (*KnowledgeFile, *ExtractionResult, error) {
	// Generate unique ID for the file
	fileID := uuid.New().String()
	
	// Create directory for the file
	filePath := filepath.Join(sm.basePath, "files", fileID)
	if err := os.MkdirAll(filePath, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Save the zip file
	zipPath := filepath.Join(filePath, "content.zip")
	outFile, err := os.Create(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()
	
	// Copy the file data
	size, err := io.Copy(outFile, fileData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save file: %w", err)
	}
	
	// Extract the zip file and track results
	extractedPath := filepath.Join(filePath, "extracted")
	extractResult := &ExtractionResult{
		Success: true,
		FilesExtracted: 0,
		MarkdownFiles: 0,
		TotalSizeBytes: 0,
	}
	
	if err := sm.extractZipFile(zipPath, extractedPath); err != nil {
		return nil, &ExtractionResult{Success: false, Error: err}, fmt.Errorf("failed to extract zip file: %w", err)
	}
	
	// Count files and gather stats
	err = filepath.Walk(extractedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Update extraction stats
		extractResult.FilesExtracted++
		extractResult.TotalSizeBytes += info.Size()
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			extractResult.MarkdownFiles++
		}
		
		return nil
	})
	
	if err != nil {
		sm.logger.Warn("Failed to gather extraction stats", "error", err)
		// Continue with the process even if stats gathering fails
	}
	
	// Create knowledge file record
	knowledgeFile := KnowledgeFile{
		ID:          fileID,
		Name:        name,
		Description: description,
		FilePath:    filePath,
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
	if err := sm.saveRegistry(); err != nil {
		return nil, extractResult, fmt.Errorf("failed to update registry: %w", err)
	}
	
	return &knowledgeFile, extractResult, nil
}

// extractZipFile extracts a zip file to the specified directory
func (sm *StorageManager) extractZipFile(zipPath, destPath string) error {
	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}
	
	// Open the zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()
	
	// Extract each file
	for _, file := range reader.File {
		// Ensure file path is safe
		filePath := filepath.Join(destPath, file.Name)
		if !strings.HasPrefix(filePath, destPath) {
			return fmt.Errorf("invalid file path in zip: %s", file.Name)
		}
		
		// Create directory for file if needed
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}
		
		// Create directory for file if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		
		// Create file
		outFile, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		
		// Open zip file entry
		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open file in zip: %w", err)
		}
		
		// Copy content
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return fmt.Errorf("failed to extract file: %w", err)
		}
	}
	
	return nil
}

// GetKnowledgeFilesForAgent returns all knowledge files associated with an agent
func (sm *StorageManager) GetKnowledgeFilesForAgent(agentID string) []KnowledgeFile {
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
func (sm *StorageManager) GetAllKnowledgeFiles() []KnowledgeFile {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	files := make([]KnowledgeFile, len(sm.registry.KnowledgeFiles))
	copy(files, sm.registry.KnowledgeFiles)
	
	return files
}

// GetKnowledgeFile returns a knowledge file by ID
func (sm *StorageManager) GetKnowledgeFile(id string) (*KnowledgeFile, error) {
	for _, file := range sm.registry.KnowledgeFiles {
		if file.ID == id {
			return &file, nil
		}
	}
	return nil, fmt.Errorf("knowledge file not found: %s", id)
}

// DeleteKnowledgeFile deletes a knowledge file by ID
func (sm *StorageManager) DeleteKnowledgeFile(id string) error {
	// Find the file in the registry
	var fileIndex int = -1
	var filePath string
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
	err := sm.saveRegistry()
	if err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// Delete the file directory if it exists
	if filePath != "" {
		err = os.RemoveAll(filePath)
		if err != nil {
			return fmt.Errorf("failed to delete file directory: %w", err)
		}
	}

	return nil
}

// GetAllAgents returns all agents
func (sm *StorageManager) GetAllAgents() []Agent {
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
func (sm *StorageManager) GetAgent(agentID string) *Agent {
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
func (sm *StorageManager) CreateAgent(id, name, description, tenantID string) (*Agent, error) {
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
	if err := sm.saveRegistry(); err != nil {
		return nil, fmt.Errorf("failed to save registry: %w", err)
	}
	
	// Return a copy without API key
	agentCopy := agent
	agentCopy.ApiKey = ""
	
	return &agentCopy, nil
}
