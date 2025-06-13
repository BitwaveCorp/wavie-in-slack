package knowledge

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
)

// Handler handles HTTP requests for knowledge management
type Handler struct {
	storageManager *StorageManager
	logger         *slog.Logger
	maxUploadSize  int64
}

// NewHandler creates a new knowledge handler
func NewHandler(storageManager *StorageManager, logger *slog.Logger) *Handler {
	return &Handler{
		storageManager: storageManager,
		logger:         logger,
		maxUploadSize:  50 * 1024 * 1024, // 50MB max upload size
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
	knowledgeFile, err := h.storageManager.StoreKnowledgeFile(name, description, agentIDs, file, "application/zip")
	if err != nil {
		h.logger.Error("Failed to add knowledge file", "error", err)
		http.Error(w, "Failed to add knowledge file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UploadResponse{
		Success: true,
		FileID:  knowledgeFile.ID,
	})
}

// handleListFiles handles listing knowledge files
func (h *Handler) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get files from storage manager
	files := h.storageManager.GetAllKnowledgeFiles()

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

// DeleteFileResponse represents a response to a delete file request
type DeleteFileResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// handleDeleteFile handles deleting a knowledge file
func (h *Handler) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req DeleteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to parse request body", "error", err)
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.ID == "" {
		http.Error(w, "File ID is required", http.StatusBadRequest)
		return
	}

	// Delete file
	err := h.storageManager.DeleteKnowledgeFile(req.ID)
	if err != nil {
		h.logger.Error("Failed to delete knowledge file", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DeleteFileResponse{
		Success: true,
	})
}

// handleListAgents handles listing agents
func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Get agents from storage manager
		agents := h.storageManager.GetAllAgents()

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
	agent, err := h.storageManager.CreateAgent(req.ID, req.Name, req.Description, req.TenantID)
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
        .form-group {
            margin-bottom: 15px;
        }
        label {
            display: block;
            margin-bottom: 5px;
        }
        input[type="text"], textarea {
            width: 100%;
            padding: 8px;
            box-sizing: border-box;
        }
        button {
            background-color: #4CAF50;
            color: white;
            padding: 10px 15px;
            border: none;
            cursor: pointer;
        }
        button:hover {
            background-color: #45a049;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 8px;
            text-align: left;
        }
        th {
            background-color: #f2f2f2;
        }
        /* Tab styles */
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
                    html += '<tr><th>Name</th><th>Description</th><th>Agents</th><th>Uploaded</th><th>Size</th><th>Actions</th></tr>';
                    
                    data.files.forEach(file => {
                        const date = new Date(file.uploaded_at);
                        const formattedDate = date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
                        const fileSize = (file.file_size / 1024).toFixed(2) + ' KB';
                        
                        html += '<tr><td>' + file.name + '</td><td>' + (file.description || '') + '</td><td>' + file.agent_ids.join(', ') + '</td><td>' + formattedDate + '</td><td>' + fileSize + '</td><td><button class="delete-btn" data-id="' + file.id + '">Delete</button></td></tr>';
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
            
            // Submit form
            fetch('/api/knowledge/upload', {
                method: 'POST',
                body: formData
            })
            .then(response => {
                if (!response.ok) {
                    throw new Error('Upload failed');
                }
                return response.json();
            })
            .then(data => {
                alert('File uploaded successfully!');
                document.getElementById('upload-form').reset();
                loadFiles();
            })
            .catch(error => {
                console.error('Error uploading file:', error);
                alert('Failed to upload file: ' + error.message);
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
                            alert('File deleted successfully!');
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
