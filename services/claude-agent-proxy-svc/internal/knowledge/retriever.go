package knowledge

import (
	"fmt"
	"io/fs"
	"log/slog"
	"math"
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

// Rough estimate of tokens per character for Claude API
const tokensPerChar = 0.25

// Maximum knowledge context size in tokens (to leave room for conversation)
const maxKnowledgeTokens = 50000

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
	
	// Track estimated token count
	tokenCount := int(math.Round(float64(len("# Knowledge Base\n\n")) * tokensPerChar))
	
	for i, doc := range content {
		// Estimate tokens for this document and headers
		docHeader := fmt.Sprintf("## Document %d\n\n", i+1)
		docFooter := "\n\n---\n\n"
		docHeaderTokens := int(math.Round(float64(len(docHeader)) * tokensPerChar))
		docFooterTokens := int(math.Round(float64(len(docFooter)) * tokensPerChar))
		docTokens := int(math.Round(float64(len(doc)) * tokensPerChar))
		
		// Check if adding this document would exceed the token limit
		if tokenCount + docHeaderTokens + docTokens + docFooterTokens > maxKnowledgeTokens {
			// Add a note that some content was truncated
			truncationNote := fmt.Sprintf("\n\n*Note: %d additional documents were omitted due to token limits.*\n", len(content)-i)
			builder.WriteString(truncationNote)
			r.logger.Warn("Knowledge context truncated due to token limit", "agent_id", agentID, 
				"included_docs", i, "total_docs", len(content), "estimated_tokens", tokenCount)
			break
		}
		
		// Add this document
		builder.WriteString(docHeader)
		builder.WriteString(doc)
		builder.WriteString(docFooter)
		
		// Update token count
		tokenCount += docHeaderTokens + docTokens + docFooterTokens
	}
	
	r.logger.Info("Knowledge context prepared", "agent_id", agentID, "docs_count", len(content), 
		"estimated_tokens", tokenCount, "estimated_chars", builder.Len())
	
	return builder.String(), nil
}
