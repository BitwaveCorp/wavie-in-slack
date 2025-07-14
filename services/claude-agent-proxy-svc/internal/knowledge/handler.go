package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc/internal/config"
)

// Handler handles HTTP requests for knowledge management
type Handler struct {
	storageBackend StorageBackend
	logger         *slog.Logger
	maxUploadSize  int64
	ragConfig      *config.RAGConfig
}

// NewHandler creates a new knowledge handler
func NewHandler(storageBackend StorageBackend, logger *slog.Logger, ragConfig *config.RAGConfig) *Handler {
	return &Handler{
		storageBackend: storageBackend,
		logger:         logger,
		maxUploadSize:  50 * 1024 * 1024, // 50MB max upload size
		ragConfig:      ragConfig,
	}
}

// RegisterRoutes registers the knowledge management routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/knowledge", h.handleUI)
	mux.HandleFunc("/api/knowledge/upload", h.handleUpload)
	mux.HandleFunc("/api/knowledge/files", h.handleListFiles)
	mux.HandleFunc("/api/knowledge/files/delete", h.handleDeleteFile)
	mux.HandleFunc("/api/knowledge/agents", h.handleListAgents)
}

// handleUpload handles file uploads
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit the request size
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)
	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		h.logger.Error("Failed to parse multipart form", "error", err)
		http.Error(w, "File too large or invalid form data", http.StatusBadRequest)
		return
	}

	// Get form values
	name := r.FormValue("name")
	description := r.FormValue("description")
	agentIDs := r.PostForm["agent_ids"]

	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if len(agentIDs) == 0 {
		http.Error(w, "At least one agent ID must be selected", http.StatusBadRequest)
		return
	}

	// Get the file
	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.Error("Failed to get file from form", "error", err)
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check file extension
	if filepath.Ext(header.Filename) != ".zip" {
		http.Error(w, "Only ZIP files are allowed", http.StatusBadRequest)
		return
	}

	// Create knowledge file
	knowledgeFile, extractionResult, err := h.storageBackend.StoreKnowledgeFile(name, description, agentIDs, file, "application/zip")
	if err != nil {
		h.logger.Error("Failed to add knowledge file", "error", err)
		http.Error(w, "Failed to add knowledge file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Send to RAG service for embedding generation if enabled
	ragResults := struct {
		Enabled      bool   `json:"enabled"`
		Processed    int    `json:"processed"`
		Successful   int    `json:"successful"`
		Failed       int    `json:"failed"`
		ErrorMessage string `json:"error_message,omitempty"`
	}{
		Enabled: h.ragConfig != nil && h.ragConfig.Enabled && h.ragConfig.URL != "",
	}

	if ragResults.Enabled {
		// Get the extracted directory path
		extractedPath := filepath.Join(knowledgeFile.FilePath, "extracted")
		h.logger.Info("Processing extracted directory for RAG service", "path", extractedPath)
		
		// For GCP storage, ensure the extracted directory is cached locally
		var localExtractedPath string
		if h.storageBackend.GetStorageType() == "gcp" {
			// Use type assertion to access the GCP-specific method
			if gcpStorage, ok := h.storageBackend.(*GCPStorageManager); ok {
				path, err := gcpStorage.ensureExtractedDirExists(knowledgeFile.FilePath)
				if err != nil {
					h.logger.Error("Failed to ensure extracted directory exists in cache", "path", extractedPath, "error", err)
					ragResults.ErrorMessage = "Failed to access extracted files: " + err.Error()
				} else {
					localExtractedPath = path
					h.logger.Info("Using cached extracted directory", "path", localExtractedPath)
				}
			}
		}
		
		// If we have a local path, process the markdown files
		if localExtractedPath != "" {
			// Walk through all files in the extracted directory
			filepath.WalkDir(localExtractedPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Skip errors and continue
				}
				
				// Skip directories
				if d.IsDir() {
					return nil
				}
				
				// Only process markdown files
				if !strings.HasSuffix(strings.ToLower(path), ".md") {
					return nil
				}
				
				// Get relative path for the file
				relPath, err := filepath.Rel(localExtractedPath, path)
				if err != nil {
					h.logger.Error("Failed to get relative path", "path", path, "error", err)
					return nil
				}
				
				// Create GCS path for the file
				gcsFilePath := fmt.Sprintf("%s/extracted/%s", knowledgeFile.FilePath, relPath)
				
				// Sanitize the relative path for use in document ID
				sanitizedPath := sanitizeDocumentID(relPath)
				
				h.logger.Info("Sending file to RAG service for embedding generation", 
					"file_path", gcsFilePath,
					"document_id", knowledgeFile.ID + "-" + sanitizedPath,
					"original_path", relPath)
				
				// Create request payload
				reqBody, err := json.Marshal(map[string]string{
					"document_id": knowledgeFile.ID + "-" + sanitizedPath,
					"file_path": gcsFilePath,
				})
				if err != nil {
					h.logger.Error("Failed to marshal RAG request", "error", err)
					ragResults.Failed++
					return nil
				}
				
				// Increment processed count
				ragResults.Processed++
				
				// Send to RAG service
				resp, err := http.Post(
					h.ragConfig.URL + "/api/documents",
					"application/json",
					bytes.NewBuffer(reqBody),
				)
				if err != nil {
					h.logger.Error("Failed to send file to RAG service", "error", err)
					ragResults.Failed++
					return nil
				}
				defer resp.Body.Close()
				
				// Check response
				if resp.StatusCode != http.StatusOK {
					respBody, _ := io.ReadAll(resp.Body)
					h.logger.Error("RAG service returned error", 
						"status", resp.Status,
						"response", string(respBody))
					ragResults.Failed++
					if ragResults.ErrorMessage == "" {
						ragResults.ErrorMessage = fmt.Sprintf("RAG service error: %s - %s", resp.Status, string(respBody))
					}
					return nil
				}
				
				h.logger.Info("Successfully sent file to RAG service", "file_path", gcsFilePath)
				ragResults.Successful++
				return nil
			})
		} else if ragResults.ErrorMessage == "" {
			ragResults.ErrorMessage = "Could not process files for RAG service: no local path available"
			h.logger.Warn(ragResults.ErrorMessage)
		}
	}

	// Create extraction details for response
	extractionDetails := &ExtractionDetails{
		Success:        extractionResult.Success,
		FilesExtracted: extractionResult.FilesExtracted,
		MarkdownFiles:  extractionResult.MarkdownFiles,
		TotalSizeBytes: extractionResult.TotalSizeBytes,
	}

	// Add error message if extraction failed
	if !extractionResult.Success && extractionResult.Error != nil {
		extractionDetails.ErrorMessage = extractionResult.Error.Error()
	}

	// Create RAG details for response
	ragDetails := struct {
		Enabled      bool   `json:"enabled"`
		Processed    int    `json:"processed"`
		Successful   int    `json:"successful"`
		Failed       int    `json:"failed"`
		ErrorMessage string `json:"error_message,omitempty"`
	}(ragResults)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		Success:    true,
		FileID:     knowledgeFile.ID,
		Extraction: extractionDetails,
		RAG:        ragDetails,
	})
}

// handleListFiles handles listing knowledge files
func (h *Handler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get files from storage manager
	files := h.storageBackend.GetAllKnowledgeFiles()

	// Return files
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListFilesResponse{
		Files: files,
	})
}

// DeleteFileRequest represents a request to delete a knowledge file
type DeleteFileRequest struct {
	ID string `json:"id"`
}

// DeleteFileResponse represents the response for file deletion
type DeleteFileResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Details string                 `json:"details,omitempty"`
	RAG     interface{}            `json:"rag,omitempty"`
}

// sanitizeDocumentID replaces characters that are not allowed in Firestore document IDs
// Firestore document IDs cannot contain: /, ., .., *, [, ], ~, or characters that match the regex __.*__
func sanitizeDocumentID(id string) string {
	// Replace forward slashes with dashes
	id = strings.ReplaceAll(id, "/", "-")
	
	// Replace periods with underscores
	id = strings.ReplaceAll(id, ".", "_")
	
	// Replace other invalid characters
	id = strings.ReplaceAll(id, "*", "_star_")
	id = strings.ReplaceAll(id, "[", "_lbracket_")
	id = strings.ReplaceAll(id, "]", "_rbracket_")
	id = strings.ReplaceAll(id, "~", "_tilde_")
	
	// Handle double underscores pattern
	if strings.Contains(id, "__") {
		id = strings.ReplaceAll(id, "__", "_underscore_underscore_")
	}
	
	return id
}

// handleDeleteFile handles deleting a knowledge file
func (h *Handler) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create a context with timeout for the entire operation
	// Increase timeout to 120 seconds for large file deletions
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// Parse request body
	var req DeleteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse request body", "error", err)
		respondWithError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.ID == "" {
		respondWithError(w, "File ID is required", http.StatusBadRequest)
		return
	}

	// Log the delete request
	h.logger.Info("Processing file deletion request", "file_id", req.ID, "remote_addr", r.RemoteAddr)

	// Create a channel to handle timeout for the delete operation
	deleteDone := make(chan struct{
		err error
		success bool
	}, 1)

	// Execute delete operation in a goroutine
	go func() {
		err := h.storageBackend.DeleteKnowledgeFile(req.ID)
		deleteDone <- struct {
			err error
			success bool
		}{err, err == nil}
	}()

	// Wait for either completion or timeout
	select {
	case result := <-deleteDone:
		if result.err != nil {
			h.logger.Error("Failed to delete knowledge file", "error", result.err, "file_id", req.ID)
			
			// Determine appropriate status code based on error
			statusCode := http.StatusInternalServerError
			errorMessage := "Failed to delete file"
			
			if strings.Contains(result.err.Error(), "not found") {
				statusCode = http.StatusNotFound
				errorMessage = "File not found"
			} else if strings.Contains(result.err.Error(), "deadline exceeded") {
				statusCode = http.StatusGatewayTimeout
				errorMessage = "Operation timed out"
			}
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(DeleteFileResponse{
				Success: false,
				Error:   errorMessage,
				Details: result.err.Error(),
			})
			return
		}
		
		// Success case
		h.logger.Info("Successfully deleted knowledge file", "file_id", req.ID)
		
		// Delete from RAG service if enabled
		ragResults := struct {
			Enabled      bool   `json:"enabled"`
			Attempted    int    `json:"attempted"`
			Successful   int    `json:"successful"`
			Failed       int    `json:"failed"`
			ErrorMessage string `json:"error_message,omitempty"`
		}{
			Enabled: h.ragConfig != nil && h.ragConfig.Enabled && h.ragConfig.URL != "",
		}
		
		if ragResults.Enabled {
			h.logger.Info("Deleting document embeddings from RAG service", "file_id", req.ID)
			
			// Check if this is a ZIP file by looking at the file extension
			isZipFile := strings.HasSuffix(strings.ToLower(req.ID), ".zip")
			
			// For ZIP files, use the prefix-based deletion to delete all extracted files
			// Use the standard document deletion endpoint for both single files and ZIP files
			// The document ID in RAG service is the same as the file ID in storage
			// This should delete all chunks associated with this document ID
			var deleteURL string
			
			// Use sanitized document ID for deletion to match the sanitized IDs used during upload
			sanitizedID := sanitizeDocumentID(req.ID)
			deleteURL = fmt.Sprintf("%s/api/documents/%s", h.ragConfig.URL, sanitizedID)
			h.logger.Info("Using standard deletion for file", "file_id", req.ID, "sanitized_id", sanitizedID)
			
			ragResults.Attempted++
			
			// We'll use a longer timeout in the background goroutine

			// Start asynchronous deletion for RAG service
			// This will allow the HTTP request to return quickly while the deletion continues in the background
			go func(fileID, deleteURL string) {
				// Create a new context with a longer timeout for the background process
				bgCtx, bgCancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer bgCancel()
				
				// Create HTTP client with increased timeout for RAG service calls
				bgClient := &http.Client{
					Timeout: 5 * time.Minute, // 5 minutes timeout for background deletion
				}
				
				// Create request with the background context
				request, err := http.NewRequestWithContext(bgCtx, "DELETE", deleteURL, nil)
				if err != nil {
					h.logger.Error("Background deletion: Failed to create request to RAG service", "error", err, "file_id", fileID)
					return
				}
				
				// Execute the request
				h.logger.Info("Background deletion: Starting RAG service deletion", "file_id", fileID)
				resp, err := bgClient.Do(request)
				if err != nil {
					h.logger.Error("Background deletion: Failed to delete from RAG service", "error", err, "file_id", fileID)
					return
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					respBody, _ := io.ReadAll(resp.Body)
					h.logger.Error("Background deletion: RAG service returned error", 
						"status", resp.Status,
						"response", string(respBody),
						"file_id", fileID)
				} else {
					h.logger.Info("Background deletion: Successfully deleted document embeddings", "file_id", fileID)
				}
			}(req.ID, deleteURL)
			
			// Mark as attempted but not yet completed
			ragResults.Attempted++
			
			// Set a message indicating that deletion is in progress
			ragResults.ErrorMessage = "Deletion started and will continue in the background"
			
			// Mark as successful since we've started the background deletion process
			ragResults.Successful++
			
			// Log that we've started the background deletion
			deleteType := "standard"
			if isZipFile {
				deleteType = "prefix-based"
			}
			h.logger.Info("Started background deletion of document embeddings", 
				"file_id", req.ID,
				"sanitized_id", sanitizedID,
				"delete_type", deleteType,
				"delete_url", deleteURL)
			
			// Add detailed log about the asynchronous process
			h.logger.Info("Asynchronous deletion details",
				"file_id", req.ID,
				"timeout", "10 minutes for background context",
				"http_timeout", "5 minutes for HTTP client",
				"process", "Background goroutine will continue deletion after HTTP response")

		}
		
		// Return a 202 Accepted status with informative message about background deletion
		responseData := DeleteFileResponse{
			Success: true,
			Message: "File successfully deleted from storage",
			Details: "RAG service deletion has been started and will continue in the background. This may take several minutes for large files with many chunks.",
			RAG:     ragResults,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(responseData)
	
	case <-ctx.Done():
		// Context timeout or cancellation
		h.logger.Error("Delete operation timed out", "file_id", req.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGatewayTimeout)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "Operation timed out",
			Details: "The delete operation took too long and timed out",
		})
	}
}

// respondWithError is a helper function to send error responses in a consistent format
func respondWithError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// respondWithJSON is a helper function to send JSON responses in a consistent format
func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// handleListAgents handles listing agents
func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Get agents from storage manager
		agents := h.storageBackend.GetAllAgents()

		// Return agents
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListAgentsResponse{
			Agents:  agents,
		})
		return
	} else if r.Method == http.MethodPost {
		h.handleCreateAgent(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// CreateAgentRequest represents a request to create a new agent
type CreateAgentRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TenantID    string `json:"tenant_id"`
}

// CreateAgentResponse represents a response to a create agent request
type CreateAgentResponse struct {
	Success bool   `json:"success"`
	Agent   *Agent `json:"agent,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleCreateAgent handles creating a new agent
func (h *Handler) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse request body", "error", err)
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.ID == "" || req.Name == "" || req.TenantID == "" {
		http.Error(w, "ID, name, and tenant_id are required", http.StatusBadRequest)
		return
	}

	// Create agent
	agent, err := h.storageBackend.CreateAgent(req.ID, req.Name, req.Description, req.TenantID)
	if err != nil {
		h.logger.Error("Failed to create agent", "error", err)
		http.Error(w, "Failed to create agent: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateAgentResponse{
		Success: true,
		AgentID: agent.ID,
	})
}

// handleUI serves the knowledge management UI
func (h *Handler) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Serve a simple HTML form for knowledge management
	w.Header().Set("Content-Type", "text/html")
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Knowledge Management</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        h1, h2 {
            color: #333;
        }
        .upload-form {
            background-color: #f5f5f5;
            padding: 20px;
            border-radius: 5px;
        .tabs {
            overflow: hidden;
            border: 1px solid #ccc;
            background-color: #f1f1f1;
            margin-bottom: 20px;
        }
        .tab-button {
            background-color: inherit;
            float: left;
            border: none;
            outline: none;
            cursor: pointer;
            padding: 14px 16px;
            transition: 0.3s;
            font-size: 16px;
        }
        .tab-button:hover {
            background-color: #ddd;
        }
        .tab-button.active {
            background-color: #4CAF50;
            color: white;
        }
        .tab-content {
            display: none;
            padding: 6px 12px;
            border: 1px solid #ccc;
            border-top: none;
            animation: fadeEffect 1s;
        }
        @keyframes fadeEffect {
            from {opacity: 0;}
            to {opacity: 1;}
        }
        .delete-btn {
            background-color: #f44336;
            color: white;
            border: none;
            padding: 5px 10px;
            border-radius: 4px;
            cursor: pointer;
        }
        .delete-btn:hover {
            background-color: #d32f2f;
        }
    </style>
</head>
<body>
    <h1>Knowledge Management</h1>
    
    <div class="tabs">
        <button class="tab-button active" onclick="openTab(event, 'knowledge-tab')">Knowledge Files</button>
        <button class="tab-button" onclick="openTab(event, 'agents-tab')">Agent Management</button>
    </div>
    
    <div id="knowledge-tab" class="tab-content" style="display: block;">
        <h2>Upload Knowledge File</h2>
        <form id="upload-form" enctype="multipart/form-data">
            <div class="form-group">
                <label for="name">Name:</label>
                <input type="text" id="name" name="name" required>
            </div>
            
            <div class="form-group">
                <label for="description">Description:</label>
                <textarea id="description" name="description" rows="3"></textarea>
            </div>
            
            <div class="form-group">
                <label for="file">File (ZIP containing markdown files):</label>
                <input type="file" id="file" name="file" accept=".zip" required>
            </div>
            
            <div id="upload-progress" style="display: none;">
                <div class="progress-container">
                    <div class="progress-bar" id="upload-progress-bar"></div>
                </div>
                <p id="upload-status">Uploading...</p>
            </div>
            
            <div id="extraction-details" style="display: none;">
                <h3>Extraction Results</h3>
                <ul>
                    <li>Files extracted: <span id="files-extracted">0</span></li>
                    <li>Markdown files: <span id="markdown-files">0</span></li>
                    <li>Total size: <span id="total-size">0</span> KB</li>
                </ul>
            </div>
            
            <div class="form-group">
                <label>Associate with Agents:</label>
                <div id="agent-checkboxes">
                    <p>Loading agents...</p>
                </div>
            </div>
            
            <button type="submit">Upload</button>
        </form>
        
        <h2>Knowledge Files</h2>
        <div id="file-list">
            <p>Loading files...</p>
        </div>
        
        <h2>Available Agents</h2>
        <div id="agent-list-knowledge">
            <p>Loading agents...</p>
        </div>
    </div>
    
    <div id="agents-tab" class="tab-content">
        <h2>Create New Agent</h2>
        <form id="create-agent-form">
            <div class="form-group">
                <label for="agent-id">ID:</label>
                <input type="text" id="agent-id" name="id" required>
            </div>
            
            <div class="form-group">
                <label for="agent-name">Name:</label>
                <input type="text" id="agent-name" name="name" required>
            </div>
            
            <div class="form-group">
                <label for="agent-description">Description:</label>
                <textarea id="agent-description" name="description" rows="3"></textarea>
            </div>
            
            <div class="form-group">
                <label for="agent-tenant">Tenant ID:</label>
                <input type="text" id="agent-tenant" name="tenant_id" required>
            </div>
            
            <button type="submit">Create Agent</button>
        </form>
        
        <h2>Agents</h2>
        <div id="agent-list">
            <p>Loading agents...</p>
        </div>
    </div>
    
    <script>
        // Tab functionality
        function openTab(evt, tabName) {
            // Hide all tab content
            const tabContents = document.getElementsByClassName("tab-content");
            for (let i = 0; i < tabContents.length; i++) {
                tabContents[i].style.display = "none";
            }
            
            // Remove active class from all tab buttons
            const tabButtons = document.getElementsByClassName("tab-button");
            for (let i = 0; i < tabButtons.length; i++) {
                tabButtons[i].className = tabButtons[i].className.replace(" active", "");
            }
            
            // Show the current tab and add active class to the button
            document.getElementById(tabName).style.display = "block";
            evt.currentTarget.className += " active";
        }
        
        // Load agents
        function loadAgents() {
            fetch('/api/knowledge/agents')
                .then(response => response.json())
                .then(data => {
                    // Populate agent checkboxes
                    const agentCheckboxes = document.getElementById('agent-checkboxes');
                    agentCheckboxes.innerHTML = '';
                    
                    data.agents.forEach(agent => {
                        const div = document.createElement('div');
                        div.innerHTML = '<input type="checkbox" id="agent-' + agent.id + '" name="agent_ids" value="' + agent.id + '" class="agent-checkbox"><label for="agent-' + agent.id + '">' + agent.name + '</label>';
                        agentCheckboxes.appendChild(div);
                    });
                    
                    // Populate agent lists (both tabs)
                    const agentList = document.getElementById('agent-list');
                    const agentListKnowledge = document.getElementById('agent-list-knowledge');
                    
                    if (data.agents.length === 0) {
                        agentList.innerHTML = '<p>No agents found.</p>';
                        agentListKnowledge.innerHTML = '<p>No agents found.</p>';
                        return;
                    }
                    
                    let html = '<table>';
                    html += '<tr><th>ID</th><th>Name</th><th>Description</th><th>Tenant</th></tr>';
                    
                    data.agents.forEach(agent => {
                        html += '<tr><td>' + agent.id + '</td><td>' + agent.name + '</td><td>' + (agent.description || '') + '</td><td>' + agent.tenant_id + '</td></tr>';
                    });
                    
                    html += '</table>';
                    agentList.innerHTML = html;
                    agentListKnowledge.innerHTML = html;
                })
                .catch(error => {
                    console.error('Error loading agents:', error);
                    document.getElementById('agent-checkboxes').innerHTML = '<p>Error loading agents.</p>';
                    document.getElementById('agent-list').innerHTML = '<p>Error loading agents.</p>';
                    document.getElementById('agent-list-knowledge').innerHTML = '<p>Error loading agents.</p>';
                });
        }
        
        // Load knowledge files
        function loadFiles() {
            fetch('/api/knowledge/files')
                .then(response => response.json())
                .then(data => {
                    const fileList = document.getElementById('file-list');
                    if (data.files.length === 0) {
                        fileList.innerHTML = '<p>No files found.</p>';
                        return;
                    }
                    
                    let html = '<table>';
                    html += '<tr><th>Name</th><th>Description</th><th>Agents</th><th>Uploaded</th><th>Size</th><th>Extraction</th><th>Actions</th></tr>';
                    
                    data.files.forEach(file => {
                        const date = new Date(file.uploaded_at);
                        const formattedDate = date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
                        const fileSize = (file.file_size / 1024).toFixed(2) + ' KB';
                        
                        // Add extraction status if available
                        let extractionStatus = '';
                        if (file.extraction) {
                            if (file.extraction.success) {
                                extractionStatus = '<span class="success-message">' + file.extraction.files_extracted + ' files extracted</span>';
                                if (file.extraction.markdown_files > 0) {
                                    extractionStatus += '<br>' + file.extraction.markdown_files + ' markdown files';
                                }
                            } else {
                                extractionStatus = '<span class="error-message">Extraction failed</span>';
                            }
                        } else if (file.name.toLowerCase().endsWith('.zip')) {
                            extractionStatus = 'Not extracted';
                        } else {
                            extractionStatus = 'N/A';
                        }
                        
                        html += '<tr><td>' + file.name + '</td><td>' + (file.description || '') + '</td><td>' + file.agent_ids.join(', ') + '</td><td>' + formattedDate + '</td><td>' + fileSize + '</td><td>' + extractionStatus + '</td><td><button class="delete-btn" data-id="' + file.id + '">Delete</button></td></tr>';
                    });
                    
                    html += '</table>';
                    fileList.innerHTML = html;
                })
                .catch(error => {
                    console.error('Error loading files:', error);
                    document.getElementById('file-list').innerHTML = '<p>Error loading files.</p>';
                });
        }
        
        // Handle file upload
        document.getElementById('upload-form').addEventListener('submit', function(e) {
            e.preventDefault();
            
            const formData = new FormData();
            formData.append('name', document.getElementById('name').value);
            formData.append('description', document.getElementById('description').value);
            formData.append('file', document.getElementById('file').files[0]);
            
            // Get selected agent IDs
            const checkboxes = document.querySelectorAll('.agent-checkbox:checked');
            const agentIds = Array.from(checkboxes).map(cb => cb.value);
            
            // Add agent IDs to form data
            agentIds.forEach(id => {
                formData.append('agent_ids', id);
            });
            
            // Show progress bar
            const progressContainer = document.getElementById('upload-progress');
            const progressBar = document.getElementById('upload-progress-bar');
            const uploadStatus = document.getElementById('upload-status');
            
            progressContainer.style.display = 'block';
            progressBar.style.width = '0%';
            uploadStatus.textContent = 'Uploading...';
            
            // Hide extraction details if previously shown
            document.getElementById('extraction-details').style.display = 'none';
            
            // Simulate progress for upload (since fetch API doesn't provide progress events easily)
            let progress = 0;
            const progressInterval = setInterval(() => {
                if (progress < 90) {
                    progress += 5;
                    progressBar.style.width = progress + '%';
                }
            }, 300);
            
            // Submit form
            fetch('/api/knowledge/upload', {
                method: 'POST',
                body: formData
            })
            .then(response => {
                clearInterval(progressInterval);
                progressBar.style.width = '100%';
                
                if (!response.ok) {
                    throw new Error('Upload failed');
                }
                return response.json();
            })
            .then(data => {
                // Update status
                uploadStatus.innerHTML = '<span class="success-message">File uploaded successfully!</span>';
                
                // Display extraction details if available
                if (data.extraction) {
                    const extractionDetails = document.getElementById('extraction-details');
                    extractionDetails.style.display = 'block';
                    
                    document.getElementById('files-extracted').textContent = data.extraction.files_extracted;
                    document.getElementById('markdown-files').textContent = data.extraction.markdown_files;
                    document.getElementById('total-size').textContent = (data.extraction.total_size_bytes / 1024).toFixed(2);
                    
                    // Show extraction status
                    if (data.extraction.success) {
                        uploadStatus.innerHTML += '<br><span class="success-message">Files extracted successfully!</span>';
                    } else {
                        uploadStatus.innerHTML += '<br><span class="error-message">Extraction failed: ' + 
                            (data.extraction.error_message || 'Unknown error') + '</span>';
                    }
                }
                
                // Display RAG processing details if available
                if (data.rag) {
                    // Create RAG details section if it doesn't exist
                    let ragDetails = document.getElementById('rag-details');
                    if (!ragDetails) {
                        ragDetails = document.createElement('div');
                        ragDetails.id = 'rag-details';
                        ragDetails.className = 'details-section';
                        ragDetails.innerHTML = '<h3>RAG Processing Results</h3>' +
                            '<ul>' +
                            '<li>Files Processed: <span id="rag-processed">0</span></li>' +
                            '<li>Successfully Embedded: <span id="rag-successful">0</span></li>' +
                            '<li>Failed: <span id="rag-failed">0</span></li>' +
                            '</ul>';
                        document.getElementById('extraction-details').after(ragDetails);
                    }
                    
                    // Update RAG stats
                    document.getElementById('rag-processed').textContent = data.rag.processed;
                    document.getElementById('rag-successful').textContent = data.rag.successful;
                    document.getElementById('rag-failed').textContent = data.rag.failed;
                    
                    // Show RAG status
                    if (data.rag.enabled) {
                        if (data.rag.successful > 0) {
                            uploadStatus.innerHTML += '<br><span class="success-message">Files processed by RAG service: ' + 
                                data.rag.successful + ' of ' + data.rag.processed + ' successful</span>';
                        }
                        if (data.rag.failed > 0) {
                            uploadStatus.innerHTML += '<br><span class="error-message">RAG processing failed for ' + 
                                data.rag.failed + ' files</span>';
                        }
                        if (data.rag.error_message) {
                            uploadStatus.innerHTML += '<br><span class="error-message">RAG error: ' + 
                                data.rag.error_message + '</span>';
                        }
                    }
                }
                
                // Reset form and reload files after a delay
                setTimeout(() => {
                    document.getElementById('upload-form').reset();
                    loadFiles();
                }, 3000);
            })
            .catch(error => {
                clearInterval(progressInterval);
                console.error('Error uploading file:', error);
                uploadStatus.innerHTML = '<span class="error-message">Failed to upload file: ' + error.message + '</span>';
            });
        });
        
        // Handle agent creation
        document.getElementById('create-agent-form').addEventListener('submit', function(e) {
            e.preventDefault();
            
            const formData = {
                id: document.getElementById('agent-id').value,
                name: document.getElementById('agent-name').value,
                description: document.getElementById('agent-description').value,
                tenant_id: document.getElementById('agent-tenant').value
            };
            
            // Submit form
            fetch('/api/knowledge/agents', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(formData)
            })
            .then(response => {
                if (!response.ok) {
                    throw new Error('Agent creation failed');
                }
                return response.json();
            })
            .then(data => {
                alert('Agent created successfully!');
                document.getElementById('create-agent-form').reset();
                loadAgents();
            })
            .catch(error => {
                console.error('Error creating agent:', error);
                alert('Failed to create agent: ' + error.message);
            });
        });
        
        // Handle file deletion
        document.addEventListener('click', function(e) {
            if (e.target && e.target.classList.contains('delete-btn')) {
                if (confirm('Are you sure you want to delete this knowledge file?')) {
                    const fileId = e.target.getAttribute('data-id');
                    
                    fetch('/api/knowledge/files/delete', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json'
                        },
                        body: JSON.stringify({ id: fileId })
                    })
                    .then(response => {
                        if (!response.ok) {
                            throw new Error('Delete failed');
                        }
                        return response.json();
                    })
                    .then(data => {
                        if (data.success) {
                            let message = 'File deleted successfully!';
                            
                            // Add RAG deletion results if available
                            if (data.rag && data.rag.enabled) {
                                if (data.rag.successful > 0) {
                                    message += '\n\nRAG service: ' + data.rag.successful + ' of ' + 
                                        data.rag.attempted + ' document embeddings deleted successfully.';
                                }
                                if (data.rag.failed > 0) {
                                    message += '\n\nRAG service: Failed to delete ' + data.rag.failed + 
                                        ' document embeddings.';
                                    if (data.rag.error_message) {
                                        message += '\nError: ' + data.rag.error_message;
                                    }
                                }
                            }
                            
                            alert(message);
                            loadFiles();
                        } else {
                            throw new Error(data.error || 'Unknown error');
                        }
                    })
                    .catch(error => {
                        console.error('Error deleting file:', error);
                        alert('Failed to delete file: ' + error.message);
                    });
                }
            }
        });
        
        // Initial load
        loadAgents();
        loadFiles();
    </script>
</body>
</html>`
	io.WriteString(w, html)
}
