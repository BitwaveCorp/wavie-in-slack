package knowledge

import (
	"encoding/json"
	"fmt"
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
	mux.HandleFunc("/api/knowledge/upload", h.handleUpload)
	mux.HandleFunc("/api/knowledge/files", h.handleListFiles)
	mux.HandleFunc("/api/knowledge/agents", h.handleListAgents)
	mux.HandleFunc("/api/knowledge/agents/create", h.handleCreateAgent)
	mux.HandleFunc("/api/knowledge/ui", h.handleUI)
}

// handleUpload handles file uploads
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request size
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	// Parse multipart form
	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		h.logger.Error("Failed to parse multipart form", "error", err)
		http.Error(w, "Request too large or invalid", http.StatusBadRequest)
		return
	}

	// Get form values
	name := r.FormValue("name")
	description := r.FormValue("description")
	agentIDs := r.Form["agent_ids"]

	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if len(agentIDs) == 0 {
		http.Error(w, "At least one agent ID is required", http.StatusBadRequest)
		return
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.Error("Failed to get file from form", "error", err)
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check file extension
	if filepath.Ext(header.Filename) != ".zip" {
		http.Error(w, "Only .zip files are allowed", http.StatusBadRequest)
		return
	}

	// Store the file
	knowledgeFile, err := h.storageManager.StoreKnowledgeFile(
		name,
		description,
		agentIDs,
		file,
		header.Header.Get("Content-Type"),
	)
	if err != nil {
		h.logger.Error("Failed to store knowledge file", "error", err)
		http.Error(w, "Failed to store file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
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

	// Get agent ID from query params
	agentID := r.URL.Query().Get("agent_id")

	var files []KnowledgeFile
	if agentID != "" {
		// Get files for specific agent
		files = h.storageManager.GetKnowledgeFilesForAgent(agentID)
	} else {
		// Get all files
		files = h.storageManager.GetAllKnowledgeFiles()
	}

	// Return files
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListFilesResponse{
		Files: files,
	})
}

// handleListAgents handles listing agents
func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all agents
	agents := h.storageManager.GetAllAgents()

	// Return agents
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListAgentsResponse{
		Agents: agents,
	})
}

// handleCreateAgent handles creating a new agent
func (h *Handler) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var agent Agent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if agent.ID == "" || agent.Name == "" || agent.TenantID == "" {
		http.Error(w, "ID, name, and tenant ID are required", http.StatusBadRequest)
		return
	}

	// Create agent
	createdAgent, err := h.storageManager.CreateAgent(
		agent.ID,
		agent.Name,
		agent.Description,
		agent.TenantID,
	)
	if err != nil {
		h.logger.Error("Failed to create agent", "error", err)
		http.Error(w, "Failed to create agent: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return created agent
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(createdAgent)
}

// handleUI serves the knowledge management UI
func (h *Handler) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Serve a simple HTML form for knowledge management
	w.Header().Set("Content-Type", "text/html")
	html := `
<!DOCTYPE html>
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
            font-weight: bold;
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
            font-size: 16px;
        }
        button:hover {
            background-color: #45a049;
        }
        .agent-checkbox {
            margin-right: 10px;
        }
        #file-list, #agent-list {
            margin-top: 20px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            text-align: left;
            padding: 8px;
            border-bottom: 1px solid #ddd;
        }
        th {
            background-color: #f2f2f2;
        }
    </style>
</head>
<body>
    <h1>Knowledge Management</h1>
    
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
            <label>Assign to Agents:</label>
            <div id="agent-checkboxes"></div>
        </div>
        
        <div class="form-group">
            <label for="file">File (ZIP format):</label>
            <input type="file" id="file" name="file" accept=".zip" required>
        </div>
        
        <button type="submit">Upload</button>
    </form>
    
    <h2>Knowledge Files</h2>
    <div id="file-list">Loading...</div>
    
    <h2>Agents</h2>
    <div id="agent-list">Loading...</div>
    
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
            <label for="tenant-id">Tenant ID:</label>
            <input type="text" id="tenant-id" name="tenantId" required>
        </div>
        
        <button type="submit">Create Agent</button>
    </form>

    <script>
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
                        div.innerHTML = `
                            <input type="checkbox" id="agent-${agent.id}" name="agent_ids" value="${agent.id}" class="agent-checkbox">
                            <label for="agent-${agent.id}">${agent.name}</label>
                        `;
                        agentCheckboxes.appendChild(div);
                    });
                    
                    // Populate agent list
                    const agentList = document.getElementById('agent-list');
                    if (data.agents.length === 0) {
                        agentList.innerHTML = '<p>No agents found.</p>';
                        return;
                    }
                    
                    let html = '<table>';
                    html += '<tr><th>ID</th><th>Name</th><th>Description</th><th>Tenant</th></tr>';
                    
                    data.agents.forEach(agent => {
                        html += `<tr>
                            <td>${agent.id}</td>
                            <td>${agent.name}</td>
                            <td>${agent.description || ''}</td>
                            <td>${agent.tenant_id}</td>
                        </tr>`;
                    });
                    
                    html += '</table>';
                    agentList.innerHTML = html;
                })
                .catch(error => {
                    console.error('Error loading agents:', error);
                    document.getElementById('agent-checkboxes').innerHTML = '<p>Error loading agents.</p>';
                    document.getElementById('agent-list').innerHTML = '<p>Error loading agents.</p>';
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
                    html += '<tr><th>Name</th><th>Description</th><th>Agents</th><th>Uploaded</th><th>Size</th></tr>';
                    
                    data.files.forEach(file => {
                        const date = new Date(file.uploaded_at);
                        const formattedDate = date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
                        const fileSize = (file.file_size / 1024).toFixed(2) + ' KB';
                        
                        html += `<tr>
                            <td>${file.name}</td>
                            <td>${file.description || ''}</td>
                            <td>${file.agent_ids.join(', ')}</td>
                            <td>${formattedDate}</td>
                            <td>${fileSize}</td>
                        </tr>`;
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
            
            // Get selected agents
            const agentCheckboxes = document.querySelectorAll('input[name="agent_ids"]:checked');
            agentCheckboxes.forEach(checkbox => {
                formData.append('agent_ids', checkbox.value);
            });
            
            fetch('/api/knowledge/upload', {
                method: 'POST',
                body: formData
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    alert('File uploaded successfully!');
                    document.getElementById('upload-form').reset();
                    loadFiles();
                } else {
                    alert('Error: ' + (data.error || 'Unknown error'));
                }
            })
            .catch(error => {
                console.error('Error uploading file:', error);
                alert('Error uploading file. See console for details.');
            });
        });
        
        // Handle agent creation
        document.getElementById('create-agent-form').addEventListener('submit', function(e) {
            e.preventDefault();
            
            const formData = {
                id: document.getElementById('agent-id').value,
                name: document.getElementById('agent-name').value,
                description: document.getElementById('agent-description').value,
                tenant_id: document.getElementById('tenant-id').value
            };
            
            fetch('/api/knowledge/agents/create', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(formData)
            })
            .then(response => response.json())
            .then(data => {
                alert('Agent created successfully!');
                document.getElementById('create-agent-form').reset();
                loadAgents();
            })
            .catch(error => {
                console.error('Error creating agent:', error);
                alert('Error creating agent. See console for details.');
            });
        });
        
        // Load data on page load
        loadAgents();
        loadFiles();
    </script>
</body>
</html>
`
	io.WriteString(w, html)
}
