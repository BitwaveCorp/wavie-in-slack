package knowledge

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	logger.Info("Creating GCP storage manager", "bucket", bucketName, "registry_key", registryKey, "temp_dir", localTempDir)
	
	logger.Info("Initializing GCP storage client")
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		logger.Error("Failed to create GCP storage client", "error", err)
		return nil, fmt.Errorf("failed to create GCP storage client: %w", err)
	}
	logger.Info("GCP storage client created successfully")

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
	sm.logger.Info("Ensuring bucket exists", "bucket", sm.bucketName)
	if err := sm.ensureBucketExists(ctx); err != nil {
		sm.logger.Error("Failed to ensure bucket exists", "error", err)
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	// Load existing registry if it exists
	sm.logger.Info("Loading registry")
	if err := sm.loadRegistry(ctx); err != nil {
		// If registry doesn't exist, create default agent
		if err == storage.ErrObjectNotExist {
			sm.logger.Info("Registry does not exist, creating default registry")
			defaultAgent := Agent{
				ID:          "wavie-bot",
				Name:        "Wavie Bot",
				Description: "Default Wavie Bot agent for Slack",
				TenantID:    "bitwave",
				CreatedAt:   time.Now(),
			}
			sm.registry.Agents = append(sm.registry.Agents, defaultAgent)
			
			if err := sm.saveRegistry(ctx); err != nil {
				sm.logger.Error("Failed to save initial registry", "error", err)
				return nil, fmt.Errorf("failed to save initial registry: %w", err)
			}
			sm.logger.Info("Initial registry created and saved successfully")
		} else {
			sm.logger.Error("Failed to load registry", "error", err)
			return nil, fmt.Errorf("failed to load registry: %w", err)
		}
	}

	return sm, nil
}

// ensureBucketExists checks if the bucket exists and creates it if it doesn't
func (sm *GCPStorageManager) ensureBucketExists(ctx context.Context) error {
	sm.logger.Info("Checking if bucket exists", "bucket", sm.bucketName)
	bucket := sm.client.Bucket(sm.bucketName)
	_, err := bucket.Attrs(ctx)
	if err == storage.ErrBucketNotExist {
		sm.logger.Info("Bucket does not exist, attempting to create", "bucket", sm.bucketName)
		if err := bucket.Create(ctx, "", nil); err != nil {
			sm.logger.Error("Failed to create bucket", "bucket", sm.bucketName, "error", err)
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		sm.logger.Info("Bucket created successfully", "bucket", sm.bucketName)
	} else if err != nil {
		sm.logger.Error("Failed to check if bucket exists", "bucket", sm.bucketName, "error", err)
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	} else {
		sm.logger.Info("Bucket exists", "bucket", sm.bucketName)
	}
	return nil
}

// loadRegistry loads the registry from GCS
func (sm *GCPStorageManager) loadRegistry(ctx context.Context) error {
	sm.logger.Info("Loading registry from GCS", "bucket", sm.bucketName, "key", sm.registryKey)
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.logger.Info("Attempting to read registry object")
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
	sm.logger.Info("Saving registry to GCS", "bucket", sm.bucketName, "key", sm.registryKey)
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.logger.Info("Creating new writer for registry object")
	writer := sm.client.Bucket(sm.bucketName).Object(sm.registryKey).NewWriter(ctx)
	defer writer.Close()

	sm.logger.Info("Encoding and writing registry data")
	if err := json.NewEncoder(writer).Encode(sm.registry); err != nil {
		sm.logger.Error("Failed to encode registry", "error", err)
		return fmt.Errorf("failed to encode registry: %w", err)
	}

	sm.logger.Info("Registry saved successfully")
	return nil
}

// ExtractionResult contains information about the extraction process
type ExtractionResult struct {
	Success          bool
	FilesExtracted   int
	MarkdownFiles    int
	TotalSizeBytes   int64
	Error            error
}

// StoreKnowledgeFile stores a knowledge file in GCS
func (sm *GCPStorageManager) StoreKnowledgeFile(name, description string, agentIDs []string, fileData io.Reader, contentType string) (*KnowledgeFile, *ExtractionResult, error) {
	ctx := context.Background()
	
	// Generate unique ID for the file
	fileID := uuid.New().String()
	
	// Path in GCS for this file
	filePath := fmt.Sprintf("files/%s", fileID)
	zipPath := fmt.Sprintf("%s/content.zip", filePath)
	
	// Create a temporary file to store the zip content
	tempFile, err := os.CreateTemp(sm.localTempDir, "upload-*.zip")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempZipPath := tempFile.Name()
	defer os.Remove(tempZipPath) // Clean up temp file when done
	
	// Copy the uploaded data to the temporary file
	size, err := io.Copy(tempFile, fileData)
	if err != nil {
		tempFile.Close()
		return nil, nil, fmt.Errorf("failed to save uploaded file: %w", err)
	}
	tempFile.Close()
	
	// Upload the zip file to GCS
	writer := sm.client.Bucket(sm.bucketName).Object(zipPath).NewWriter(ctx)
	writer.ContentType = contentType
	
	zipFile, err := os.Open(tempZipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open temporary zip file: %w", err)
	}
	defer zipFile.Close()
	
	_, err = io.Copy(writer, zipFile)
	if err != nil {
		writer.Close()
		return nil, nil, fmt.Errorf("failed to upload file to GCS: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to finalize GCS upload: %w", err)
	}
	
	// Extract the zip file and upload the contents to GCS
	extractionResult, err := sm.extractZipFile(ctx, tempZipPath, filePath)
	if err != nil {
		sm.logger.Error("Failed to extract zip file", "error", err)
		// Continue with the process even if extraction fails
		extractionResult = &ExtractionResult{
			Success: false,
			Error:   err,
		}
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
		return nil, extractionResult, fmt.Errorf("failed to update registry: %w", err)
	}
	
	return &knowledgeFile, extractionResult, nil
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
	// Create a context with timeout to prevent hanging operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Find the file in the registry
	var fileIndex int = -1
	var filePath string
	var fileName string
	
	// Use read lock first to find the file
	sm.mutex.RLock()
	for i, file := range sm.registry.KnowledgeFiles {
		if file.ID == id {
			fileIndex = i
			filePath = file.FilePath
			fileName = file.Name
			break
		}
	}
	sm.mutex.RUnlock()

	if fileIndex == -1 {
		return fmt.Errorf("knowledge file not found: %s", id)
	}

	// Log the deletion attempt
	sm.logger.Info("Attempting to delete knowledge file", 
		"file_id", id, 
		"file_name", fileName, 
		"file_path", filePath)

	// Now acquire write lock to update the registry
	sm.mutex.Lock()
	
	// Double-check that the file still exists (in case of concurrent deletions)
	fileStillExists := false
	for i, file := range sm.registry.KnowledgeFiles {
		if file.ID == id {
			fileIndex = i
			fileStillExists = true
			break
		}
	}
	
	if !fileStillExists {
		sm.mutex.Unlock()
		sm.logger.Warn("File was already deleted from registry", "file_id", id)
		return nil
	}

	// Remove the file from the registry
	sm.registry.KnowledgeFiles = append(
		sm.registry.KnowledgeFiles[:fileIndex],
		sm.registry.KnowledgeFiles[fileIndex+1:]...,
	)

	// Save the updated registry
	err := sm.saveRegistry(ctx)
	if err != nil {
		// Restore the registry if save fails
		sm.logger.Error("Failed to save registry after file deletion", "error", err)
		sm.mutex.Unlock()
		return fmt.Errorf("failed to save registry: %w", err)
	}
	
	// Release the mutex after registry is updated
	sm.mutex.Unlock()

	// Delete all objects with this prefix from GCS
	if filePath != "" {
		bucket := sm.client.Bucket(sm.bucketName)
		it := bucket.Objects(ctx, &storage.Query{Prefix: filePath})
		
		deleteCount := 0
		deleteErrors := 0
		
		for {
			// Check if context is done (timeout or cancellation)
			select {
			case <-ctx.Done():
				sm.logger.Warn("Context deadline exceeded while deleting objects", 
					"file_id", id, 
					"deleted_count", deleteCount, 
					"error_count", deleteErrors)
				return ctx.Err()
			default:
				// Continue processing
			}
			
			attrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				sm.logger.Error("Error listing objects to delete", "error", err)
				deleteErrors++
				continue // Continue with other objects instead of failing completely
			}
			
			// Delete the object
			if err := bucket.Object(attrs.Name).Delete(ctx); err != nil {
				sm.logger.Error("Failed to delete object", "object", attrs.Name, "error", err)
				deleteErrors++
			} else {
				deleteCount++
			}
		}
		
		sm.logger.Info("Completed file deletion from storage", 
			"file_id", id, 
			"deleted_count", deleteCount, 
			"error_count", deleteErrors)
		
		// Even if some objects failed to delete, we consider the operation successful
		// since the registry has been updated
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

// GetStorageType returns the type of storage backend
func (sm *GCPStorageManager) GetStorageType() string {
	return "gcp"
}

// ensureLocalCopy ensures that a file exists in the local cache, downloading it from GCP if necessary
func (sm *GCPStorageManager) ensureLocalCopy(gcsPath string) (string, error) {
	// Convert GCS path to local cache path
	localPath := filepath.Join(sm.localTempDir, gcsPath)
	
	// Check if file exists in local cache
	if _, err := os.Stat(localPath); err == nil {
		// File exists in cache
		sm.logger.Debug("Using cached file", "path", localPath)
		return localPath, nil
	}
	
	// File doesn't exist in cache, download from GCP
	sm.logger.Info("File not in cache, downloading from GCP", "path", gcsPath)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory for cached file: %w", err)
	}
	
	// Download file from GCP
	ctx := context.Background()
	reader, err := sm.client.Bucket(sm.bucketName).Object(gcsPath).NewReader(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get file from GCP: %w", err)
	}
	defer reader.Close()
	
	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()
	
	// Copy content
	if _, err := io.Copy(file, reader); err != nil {
		os.Remove(localPath) // Clean up partial file
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	
	sm.logger.Info("Successfully downloaded file to cache", "gcs_path", gcsPath, "local_path", localPath)
	return localPath, nil
}

// ensureExtractedDirExists ensures that an extracted directory exists in the local cache
// If it doesn't exist, it will download all files from the GCP path to the local cache
func (sm *GCPStorageManager) ensureExtractedDirExists(filePath string) (string, error) {
	// Path to the extracted directory in GCS
	extractedGCSPath := fmt.Sprintf("%s/extracted", filePath)
	
	// Local path for the extracted directory
	localExtractedPath := filepath.Join(sm.localTempDir, extractedGCSPath)
	
	// Check if directory exists in local cache
	if _, err := os.Stat(localExtractedPath); err == nil {
		// Directory exists in cache
		sm.logger.Debug("Using cached extracted directory", "path", localExtractedPath)
		return localExtractedPath, nil
	}
	
	// Directory doesn't exist, create it
	if err := os.MkdirAll(localExtractedPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create extracted directory: %w", err)
	}
	
	// List all objects in the extracted directory in GCS
	sm.logger.Info("Downloading extracted directory from GCP", "path", extractedGCSPath)
	ctx := context.Background()
	it := sm.client.Bucket(sm.bucketName).Objects(ctx, &storage.Query{Prefix: extractedGCSPath + "/"})
	
	// Download each file
	fileCount := 0
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error listing objects in extracted directory: %w", err)
		}
		
		// Skip the directory itself
		if attrs.Name == extractedGCSPath+"/" {
			continue
		}
		
		// Download the file
		_, err = sm.ensureLocalCopy(attrs.Name)
		if err != nil {
			sm.logger.Warn("Failed to download file from extracted directory", "file", attrs.Name, "error", err)
			continue
		}
		
		fileCount++
	}
	
	sm.logger.Info("Successfully downloaded extracted directory", "path", extractedGCSPath, "file_count", fileCount)
	return localExtractedPath, nil
}

// extractZipFile extracts a zip file and uploads the contents to GCS
func (sm *GCPStorageManager) extractZipFile(ctx context.Context, zipPath, gcsFilePath string) (*ExtractionResult, error) {
	// Create a temporary directory for extraction
	tempExtractDir, err := os.MkdirTemp(sm.localTempDir, "extract-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary extraction directory: %w", err)
	}
	defer os.RemoveAll(tempExtractDir) // Clean up when done

	// Open the zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Initialize extraction result
	result := &ExtractionResult{
		Success:        true,
		FilesExtracted: 0,
		MarkdownFiles:  0,
		TotalSizeBytes: 0,
	}

	// Extract each file
	for _, file := range reader.File {
		// Ensure file path is safe
		filePath := filepath.Join(tempExtractDir, file.Name)
		if !strings.HasPrefix(filePath, tempExtractDir) {
			return nil, fmt.Errorf("invalid file path in zip: %s", file.Name)
		}

		// Create directory for file if needed
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Create directory for file if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}

		// Create file
		outFile, err := os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}

		// Open zip file entry
		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return nil, fmt.Errorf("failed to open file in zip: %w", err)
		}

		// Copy content
		size, err := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to extract file: %w", err)
		}

		// Update extraction stats
		result.FilesExtracted++
		result.TotalSizeBytes += size
		if strings.HasSuffix(strings.ToLower(file.Name), ".md") {
			result.MarkdownFiles++
		}
	}

	// Upload extracted files to GCS
	extractedGCSPath := fmt.Sprintf("%s/extracted", gcsFilePath)
	err = filepath.Walk(tempExtractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(tempExtractDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Create GCS object path
		gcsObjectPath := fmt.Sprintf("%s/%s", extractedGCSPath, relPath)

		// Open the file
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file for upload: %w", err)
		}
		defer file.Close()

		// Upload to GCS
		writer := sm.client.Bucket(sm.bucketName).Object(gcsObjectPath).NewWriter(ctx)
		
		// Set content type based on file extension
		contentType := "application/octet-stream"
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			contentType = "text/markdown"
		} else if strings.HasSuffix(strings.ToLower(path), ".txt") {
			contentType = "text/plain"
		}
		writer.ContentType = contentType

		// Copy file content to GCS
		if _, err := io.Copy(writer, file); err != nil {
			writer.Close()
			return fmt.Errorf("failed to upload extracted file to GCS: %w", err)
		}

		// Close the writer
		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to finalize extracted file upload: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload extracted files: %w", err)
	}

	return result, nil
}
