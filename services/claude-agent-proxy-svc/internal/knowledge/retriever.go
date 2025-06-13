package knowledge

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Retriever provides access to knowledge files for agents
type Retriever struct {
	storageManager *StorageManager
	logger         *slog.Logger
	cache          map[string][]string // Cache of agent ID to markdown content
	mutex          sync.RWMutex
}

// NewRetriever creates a new knowledge retriever
func NewRetriever(storageManager *StorageManager, logger *slog.Logger) *Retriever {
	return &Retriever{
		storageManager: storageManager,
		logger:         logger,
		cache:          make(map[string][]string),
		mutex:          sync.RWMutex{},
	}
}

// GetKnowledgeForAgent retrieves all knowledge content for an agent
func (r *Retriever) GetKnowledgeForAgent(agentID string) ([]string, error) {
	// Check cache first
	r.mutex.RLock()
	if content, ok := r.cache[agentID]; ok {
		r.mutex.RUnlock()
		return content, nil
	}
	r.mutex.RUnlock()

	// Get knowledge files for agent
	files := r.storageManager.GetKnowledgeFilesForAgent(agentID)
	if len(files) == 0 {
		return nil, nil
	}

	// Load content from all files
	var allContent []string
	for _, file := range files {
		// Get markdown files from extracted directory
		extractedPath := filepath.Join(file.FilePath, "extracted")
		
		// Walk through all files in the extracted directory
		err := filepath.WalkDir(extractedPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			
			// Skip directories
			if d.IsDir() {
				return nil
			}
			
			// Only process markdown files
			if !strings.HasSuffix(strings.ToLower(path), ".md") {
				return nil
			}
			
			// Read file content
			content, err := os.ReadFile(path)
			if err != nil {
				r.logger.Error("Failed to read markdown file", "path", path, "error", err)
				return nil // Continue with other files
			}
			
			// Add file content to result
			allContent = append(allContent, string(content))
			return nil
		})
		
		if err != nil {
			r.logger.Error("Failed to walk extracted directory", "path", extractedPath, "error", err)
			continue // Try next file
		}
	}

	// Update cache
	r.mutex.Lock()
	r.cache[agentID] = allContent
	r.mutex.Unlock()

	return allContent, nil
}

// ClearCache clears the content cache
func (r *Retriever) ClearCache() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.cache = make(map[string][]string)
}

// ClearAgentCache clears the cache for a specific agent
func (r *Retriever) ClearAgentCache(agentID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.cache, agentID)
}

// GetKnowledgeContext returns a formatted context string with all knowledge for an agent
func (r *Retriever) GetKnowledgeContext(agentID string) (string, error) {
	content, err := r.GetKnowledgeForAgent(agentID)
	if err != nil {
		return "", fmt.Errorf("failed to get knowledge for agent: %w", err)
	}
	
	if len(content) == 0 {
		return "", nil
	}
	
	// Join all content with separators
	var builder strings.Builder
	builder.WriteString("# Knowledge Base\n\n")
	
	for i, doc := range content {
		builder.WriteString(fmt.Sprintf("## Document %d\n\n", i+1))
		builder.WriteString(doc)
		builder.WriteString("\n\n---\n\n")
	}
	
	return builder.String(), nil
}
