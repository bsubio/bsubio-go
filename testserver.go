package bsubio

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockServer provides a mock bsub.io server for testing
type MockServer struct {
	*httptest.Server
	jobs   map[uuid.UUID]*Job
	mu     sync.RWMutex
	delays map[string]time.Duration // Optional delays for specific operations
}

// NewMockServer creates a new mock bsub.io server
func NewMockServer() *MockServer {
	ms := &MockServer{
		jobs:   make(map[uuid.UUID]*Job),
		delays: make(map[string]time.Duration),
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(ms.handler))
	return ms
}

// GetJob returns a job by ID (for testing inspection)
func (ms *MockServer) GetJob(jobID uuid.UUID) *Job {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.jobs[jobID]
}

func (ms *MockServer) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check for delays
	ms.mu.RLock()
	for op, delay := range ms.delays {
		if strings.Contains(r.URL.Path, op) {
			time.Sleep(delay)
			break
		}
	}
	ms.mu.RUnlock()

	switch {
	case r.Method == "POST" && r.URL.Path == "/v1/jobs":
		ms.handleCreateJob(w, r)

	case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
		ms.handleUpload(w, r)

	case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
		ms.handleSubmit(w, r)

	case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/") && strings.Contains(r.URL.Path, "/output"):
		ms.handleGetOutput(w, r)

	case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/") && strings.Contains(r.URL.Path, "/logs"):
		ms.handleGetLogs(w, r)

	case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/"):
		ms.handleGetJob(w, r)

	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func (ms *MockServer) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	jobID := uuid.New()
	status := JobStatusCreated
	uploadToken := uuid.New().String()
	now := time.Now()
	userID := "test-user-id"
	dataSize := int64(0)

	job := &Job{
		Id:          &jobID,
		Type:        &req.Type,
		Status:      &status,
		CreatedAt:   &now,
		UpdatedAt:   &now,
		UserId:      &userID,
		UploadToken: &uploadToken,
		DataSize:    &dataSize,
	}

	ms.mu.Lock()
	ms.jobs[jobID] = job
	ms.mu.Unlock()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":    job,
		"success": true,
	})
}

func (ms *MockServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /v1/upload/{uploadToken}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid upload path", http.StatusBadRequest)
		return
	}

	// Read the uploaded data
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read upload", http.StatusBadRequest)
		return
	}

	// Update job status to loaded
	ms.mu.Lock()
	for _, job := range ms.jobs {
		if job.UploadToken != nil {
			status := JobStatusLoaded
			job.Status = &status
			dataSize := int64(len(data))
			job.DataSize = &dataSize
			break
		}
	}
	ms.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data_size": len(data),
		"message":   "Upload successful",
	})
}

func (ms *MockServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /v1/jobs/{jobId}/submit
	parts := strings.Split(r.URL.Path, "/")
	var jobID uuid.UUID
	for i, part := range parts {
		if part == "jobs" && i+1 < len(parts) {
			parsed, err := uuid.Parse(parts[i+1])
			if err == nil {
				jobID = parsed
			}
			break
		}
	}

	ms.mu.Lock()
	job, exists := ms.jobs[jobID]
	if !exists {
		ms.mu.Unlock()
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Simulate job processing - for test job types, mark as finished immediately
	// For other types, mark as pending and will need to be polled
	status := JobStatusFinished
	if job.Type != nil {
		switch *job.Type {
		case "test/linecount":
			status = JobStatusFinished
		default:
			status = JobStatusPending
		}
	}
	job.Status = &status
	now := time.Now()
	job.UpdatedAt = &now
	ms.mu.Unlock()

	// Return simple success response (matching real API)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Job submitted successfully",
	})
}

func (ms *MockServer) handleGetJob(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /v1/jobs/{jobId}
	parts := strings.Split(r.URL.Path, "/")
	var jobID uuid.UUID
	for i, part := range parts {
		if part == "jobs" && i+1 < len(parts) {
			// Remove any query parameters or additional path segments
			idPart := strings.Split(parts[i+1], "?")[0]
			parsed, err := uuid.Parse(idPart)
			if err == nil {
				jobID = parsed
			}
			break
		}
	}

	ms.mu.RLock()
	job, exists := ms.jobs[jobID]
	ms.mu.RUnlock()

	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":    job,
		"success": true,
	})
}

func (ms *MockServer) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	// For mock server, return simple output based on job type
	parts := strings.Split(r.URL.Path, "/")
	var jobID uuid.UUID
	for i, part := range parts {
		if part == "jobs" && i+1 < len(parts) {
			parsed, err := uuid.Parse(parts[i+1])
			if err == nil {
				jobID = parsed
			}
			break
		}
	}

	ms.mu.RLock()
	job, exists := ms.jobs[jobID]
	ms.mu.RUnlock()

	if !exists || job.Status == nil || *job.Status != JobStatusFinished {
		http.Error(w, "Output not available", http.StatusNotFound)
		return
	}

	// Generate mock output based on job type
	var output string
	if job.Type != nil {
		switch *job.Type {
		case "test/linecount":
			output = "5"
		default:
			output = "mock output"
		}
	} else {
		output = "mock output"
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(output))
}

func (ms *MockServer) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	var jobID uuid.UUID
	for i, part := range parts {
		if part == "jobs" && i+1 < len(parts) {
			parsed, err := uuid.Parse(parts[i+1])
			if err == nil {
				jobID = parsed
			}
			break
		}
	}

	ms.mu.RLock()
	job, exists := ms.jobs[jobID]
	ms.mu.RUnlock()

	if !exists {
		http.Error(w, "Logs not available", http.StatusNotFound)
		return
	}

	logs := "Mock job processing logs"
	if job.Type != nil {
		logs = "Processing " + *job.Type + " job\nCompleted successfully"
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(logs))
}
