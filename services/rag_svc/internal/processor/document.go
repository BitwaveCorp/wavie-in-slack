package processor

import (
	"context"
	"fmt"
	"io/ioutil"
	"unicode"

	"cloud.google.com/go/storage"
)

// DocumentProcessor handles document processing operations
type DocumentProcessor struct {
	bucketName   string
	storageClient *storage.Client
	chunkSize    int
	chunkOverlap int
}

// NewDocumentProcessor creates a new document processor
func NewDocumentProcessor(ctx context.Context, bucketName string, chunkSize, chunkOverlap int) (*DocumentProcessor, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %v", err)
	}

	// Use default values if not provided
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if chunkOverlap <= 0 {
		chunkOverlap = 200
	}

	return &DocumentProcessor{
		bucketName:   bucketName,
		storageClient: client,
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}, nil
}

// Close closes the document processor
func (p *DocumentProcessor) Close() error {
	return p.storageClient.Close()
}

// GetDocument retrieves a document from GCP Storage
func (p *DocumentProcessor) GetDocument(ctx context.Context, objectPath string) (string, error) {
	bucket := p.storageClient.Bucket(p.bucketName)
	obj := bucket.Object(objectPath)
	
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open document: %v", err)
	}
	defer reader.Close()
	
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read document: %v", err)
	}
	
	return string(data), nil
}

// ChunkDocument splits a document into overlapping chunks
func (p *DocumentProcessor) ChunkDocument(text string) []string {
	var chunks []string
	
	// Simple chunking by characters with overlap
	textRunes := []rune(text)
	textLength := len(textRunes)
	
	// If text is smaller than chunk size, return it as a single chunk
	if textLength <= p.chunkSize {
		return []string{text}
	}
	
	// Split text into chunks with overlap
	for i := 0; i < textLength; i += (p.chunkSize - p.chunkOverlap) {
		end := i + p.chunkSize
		if end > textLength {
			end = textLength
		}
		
		// Try to find a good breaking point (end of sentence or paragraph)
		if end < textLength {
			// Look for paragraph break
			for j := 0; j < 100 && end+j < textLength; j++ {
				if textRunes[end+j] == '\n' && j > 0 && textRunes[end+j-1] == '\n' {
					end = end + j + 1
					break
				}
			}
			
			// If no paragraph break found, look for sentence break
			for j := 0; j < 50 && end+j < textLength; j++ {
				if (textRunes[end+j] == '.' || textRunes[end+j] == '!' || textRunes[end+j] == '?') && 
				   j+1 < textLength && unicode.IsSpace(textRunes[end+j+1]) {
					end = end + j + 2
					break
				}
			}
		}
		
		chunk := string(textRunes[i:end])
		chunks = append(chunks, chunk)
		
		if end == textLength {
			break
		}
	}
	
	return chunks
}

// EstimateTokenCount estimates the number of tokens in a text
// This is a rough estimation based on the average ratio of tokens to characters
func (p *DocumentProcessor) EstimateTokenCount(text string) int {
	// A rough estimate: 1 token is about 4 characters in English
	return len(text) / 4
}

// ProcessMarkdown processes markdown files specifically
// This can be extended to handle markdown formatting, extract headers, etc.
func (p *DocumentProcessor) ProcessMarkdown(text string) []string {
	// For now, just use the standard chunking
	// In the future, this could be enhanced to be markdown-aware
	return p.ChunkDocument(text)
}
