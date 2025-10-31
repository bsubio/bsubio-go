package bsubio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test job
func createTestJob(status JobStatus) *Job {
	jobID := uuid.New()
	jobType := "pandoc_md"
	dataSize := int64(1024)
	uploadToken := "test-upload-token"
	now := time.Now()
	userID := "test-user-id"

	job := &Job{
		Id:          &jobID,
		Type:        &jobType,
		Status:      &status,
		DataSize:    &dataSize,
		CreatedAt:   &now,
		UpdatedAt:   &now,
		UserId:      &userID,
		UploadToken: &uploadToken,
	}

	return job
}

// Helper function to encode job response with proper wrapper
func encodeJobResponse(w http.ResponseWriter, job *Job) {
	response := map[string]interface{}{
		"data":    job,
		"success": true,
	}
	json.NewEncoder(w).Encode(response)
}

// TestNewBsubClient tests client initialization
func TestNewBsubClient(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with defaults",
			config: Config{
				APIKey: "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "valid config with custom base URL",
			config: Config{
				APIKey:  "test-api-key",
				BaseURL: "https://custom.bsub.io",
			},
			wantErr: false,
		},
		{
			name: "valid config with custom HTTP client",
			config: Config{
				APIKey:     "test-api-key",
				HTTPClient: &http.Client{Timeout: 10 * time.Second},
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: Config{
				BaseURL: "https://app.bsub.io",
			},
			wantErr:     true,
			errContains: "API key is required",
		},
		{
			name: "empty API key",
			config: Config{
				APIKey: "",
			},
			wantErr:     true,
			errContains: "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewBsubClient(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
				require.NotNil(t, client.ClientWithResponses)
			}
		})
	}
}

// TestNewBsubClient_AuthInterceptor verifies that the auth interceptor adds Bearer token
func TestNewBsubClient_AuthInterceptor(t *testing.T) {
	// Create a test server that checks for auth header
	authHeaderReceived := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaderReceived = r.Header.Get("Authorization")
		job := createTestJob(JobStatusCreated)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		encodeJobResponse(w, job)
	}))
	defer server.Close()

	// Create client with test server URL
	apiKey := "test-api-key-123"
	client, err := NewBsubClient(Config{
		APIKey:  apiKey,
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	// Make a request
	ctx := context.Background()
	reqBody := CreateJobJSONRequestBody{Type: "pandoc_md"}
	_, _ = client.CreateJobWithResponse(ctx, reqBody)

	// Verify auth header was set correctly
	assert.Equal(t, "Bearer "+apiKey, authHeaderReceived)
}

// TestCreateAndSubmitJob tests the job creation and submission flow
func TestCreateAndSubmitJob(t *testing.T) {
	t.Run("successful job creation and submission", func(t *testing.T) {
		jobID := uuid.New()
		uploadToken := "test-upload-token"
		callSequence := []string{}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.Method == "POST" && r.URL.Path == "/v1/jobs":
				// Create job
				callSequence = append(callSequence, "create")
				job := createTestJob(JobStatusCreated)
				job.Id = &jobID
				job.UploadToken = &uploadToken
				w.WriteHeader(http.StatusCreated)
				encodeJobResponse(w, job)

			case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
				// Upload data
				callSequence = append(callSequence, "upload")
				assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data_size": 1024,
					"message":   "Upload successful",
				})

			case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
				// Submit job
				callSequence = append(callSequence, "submit")
				job := createTestJob(JobStatusPending)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)

			default:
				t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		data := bytes.NewReader([]byte("test data content"))
		job, err := client.CreateAndSubmitJob(ctx, "pandoc_md", data)

		require.NoError(t, err)
		require.NotNil(t, job)
		assert.Equal(t, jobID, *job.Id)
		assert.Equal(t, []string{"create", "upload", "submit"}, callSequence)
	})

	t.Run("job creation fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		data := bytes.NewReader([]byte("test data"))
		job, err := client.CreateAndSubmitJob(ctx, "pandoc_md", data)

		require.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "failed to create job")
	})

	t.Run("upload fails", func(t *testing.T) {
		jobID := uuid.New()
		uploadToken := "test-upload-token"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.Method == "POST" && r.URL.Path == "/v1/jobs" {
				job := createTestJob(JobStatusCreated)
				job.Id = &jobID
				job.UploadToken = &uploadToken
				w.WriteHeader(http.StatusCreated)
				encodeJobResponse(w, job)
			} else if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/") {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Upload failed"))
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		data := bytes.NewReader([]byte("test data"))
		job, err := client.CreateAndSubmitJob(ctx, "pandoc_md", data)

		require.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "failed to upload data")
	})

	t.Run("submit fails", func(t *testing.T) {
		jobID := uuid.New()
		uploadToken := "test-upload-token"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.Method == "POST" && r.URL.Path == "/v1/jobs":
				job := createTestJob(JobStatusCreated)
				job.Id = &jobID
				job.UploadToken = &uploadToken
				w.WriteHeader(http.StatusCreated)
				encodeJobResponse(w, job)
			case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data_size": 1024,
					"message":   "Upload successful",
				})
			case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte("Job already submitted"))
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		data := bytes.NewReader([]byte("test data"))
		job, err := client.CreateAndSubmitJob(ctx, "pandoc_md", data)

		require.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "failed to submit job")
	})
}

// TestWaitForJob tests the polling mechanism
func TestWaitForJob(t *testing.T) {
	t.Run("job finishes successfully", func(t *testing.T) {
		jobID := uuid.New()
		pollCount := 0
		statuses := []JobStatus{JobStatusPending, JobStatusProcessing, JobStatusFinished}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/") {
				status := statuses[pollCount]
				if pollCount < len(statuses)-1 {
					pollCount++
				}

				job := createTestJob(status)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		job, err := client.WaitForJob(ctx, jobID)

		require.NoError(t, err)
		require.NotNil(t, job)
		assert.Equal(t, JobStatusFinished, *job.Status)
		assert.GreaterOrEqual(t, pollCount, 2, "should have polled multiple times")
	})

	t.Run("job fails", func(t *testing.T) {
		jobID := uuid.New()
		errorCode := "PROCESSING_ERROR"
		errorMsg := "Failed to process file"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			job := createTestJob(JobStatusFailed)
			job.Id = &jobID
			job.ErrorCode = &errorCode
			job.ErrorMessage = &errorMsg
			w.WriteHeader(http.StatusOK)
			encodeJobResponse(w, job)
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		job, err := client.WaitForJob(ctx, jobID)

		require.NoError(t, err)
		require.NotNil(t, job)
		assert.Equal(t, JobStatusFailed, *job.Status)
		assert.Equal(t, errorCode, *job.ErrorCode)
		assert.Equal(t, errorMsg, *job.ErrorMessage)
	})

	t.Run("context cancellation", func(t *testing.T) {
		jobID := uuid.New()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Always return processing status
			job := createTestJob(JobStatusProcessing)
			job.Id = &jobID
			w.WriteHeader(http.StatusOK)
			encodeJobResponse(w, job)
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		job, err := client.WaitForJob(ctx, jobID)

		require.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "context")
	})

	t.Run("HTTP error during polling", func(t *testing.T) {
		jobID := uuid.New()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server error"))
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		job, err := client.WaitForJob(ctx, jobID)

		require.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "failed to get job")
	})
}

// TestGetJobResult tests result retrieval
func TestGetJobResult(t *testing.T) {
	t.Run("successful result retrieval", func(t *testing.T) {
		jobID := uuid.New()
		outputData := []byte("# Converted Markdown\n\nThis is the output.")
		logsData := "Processing started\nProcessing completed"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == "GET" && strings.Contains(r.URL.Path, "/output"):
				w.Header().Set("Content-Type", "text/markdown")
				w.WriteHeader(http.StatusOK)
				w.Write(outputData)

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/logs"):
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(logsData))

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/"):
				job := createTestJob(JobStatusFinished)
				job.Id = &jobID
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		result, err := client.GetJobResult(ctx, jobID)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, jobID, *result.Job.Id)
		assert.Equal(t, outputData, result.Output)
		assert.Equal(t, logsData, result.Logs)
	})

	t.Run("failed job retrieval", func(t *testing.T) {
		jobID := uuid.New()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Job not found"))
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		result, err := client.GetJobResult(ctx, jobID)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get job")
	})

	t.Run("output retrieval returns non-200 - handled gracefully", func(t *testing.T) {
		jobID := uuid.New()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(r.URL.Path, "/output") {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("Output not found"))
			} else if strings.Contains(r.URL.Path, "/v1/jobs/") {
				job := createTestJob(JobStatusFinished)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		result, err := client.GetJobResult(ctx, jobID)

		// Should succeed even if output returns non-200 - output is optional
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Output) // Output should be empty when retrieval fails
	})

	t.Run("logs retrieval fails gracefully", func(t *testing.T) {
		jobID := uuid.New()
		outputData := []byte("Output data")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.Contains(r.URL.Path, "/output"):
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusOK)
				w.Write(outputData)

			case strings.Contains(r.URL.Path, "/logs"):
				w.WriteHeader(http.StatusNotFound)

			case strings.Contains(r.URL.Path, "/v1/jobs/"):
				job := createTestJob(JobStatusFinished)
				job.Id = &jobID
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		result, err := client.GetJobResult(ctx, jobID)

		// Should succeed even if logs fail - logs are optional
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, outputData, result.Output)
		assert.Empty(t, result.Logs) // Logs should be empty when retrieval fails
	})
}

// TestProcess tests end-to-end processing with reader
func TestProcess(t *testing.T) {
	t.Run("successful processing", func(t *testing.T) {
		jobID := uuid.New()
		inputData := []byte("Test input data")
		outputData := []byte("# Processed Output")
		logsData := "Processing completed successfully"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.Method == "POST" && r.URL.Path == "/v1/jobs":
				job := createTestJob(JobStatusCreated)
				job.Id = &jobID
				w.WriteHeader(http.StatusCreated)
				encodeJobResponse(w, job)

			case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data_size": 1024,
					"message":   "Upload successful",
				})

			case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
				job := createTestJob(JobStatusFinished)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/output"):
				w.Header().Set("Content-Type", "text/markdown")
				w.WriteHeader(http.StatusOK)
				w.Write(outputData)

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/logs"):
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(logsData))

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/"):
				job := createTestJob(JobStatusFinished)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		data := bytes.NewReader(inputData)
		result, err := client.Process(ctx, "pandoc_md", data)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.Equal(t, outputData, result.Output)
		assert.Equal(t, logsData, result.Logs)
	})

	t.Run("job fails during processing", func(t *testing.T) {
		jobID := uuid.New()
		errorCode := "TIMEOUT"
		errorMsg := "Processing timeout exceeded"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.Method == "POST" && r.URL.Path == "/v1/jobs":
				job := createTestJob(JobStatusCreated)
				job.Id = &jobID
				w.WriteHeader(http.StatusCreated)
				encodeJobResponse(w, job)

			case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data_size": 1024,
					"message":   "Upload successful",
				})

			case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
				job := createTestJob(JobStatusFailed)
				job.Id = &jobID
				job.ErrorCode = &errorCode
				job.ErrorMessage = &errorMsg
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/"):
				job := createTestJob(JobStatusFailed)
				job.Id = &jobID
				job.ErrorCode = &errorCode
				job.ErrorMessage = &errorMsg
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		data := bytes.NewReader([]byte("test data"))
		result, err := client.Process(ctx, "pandoc_md", data)

		require.Error(t, err)
		// Result is returned even on failure (contains job metadata)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFailed, *result.Job.Status)
		assert.Contains(t, err.Error(), "job failed")
		assert.Contains(t, err.Error(), errorMsg)
	})
}

// TestCreateAndSubmitJobFromFile tests file-based job submission
func TestCreateAndSubmitJobFromFile(t *testing.T) {
	t.Run("successful file processing", func(t *testing.T) {
		// Create temporary test file
		tmpDir := t.TempDir()
		testFilePath := filepath.Join(tmpDir, "test.pdf")
		testContent := []byte("PDF content simulation")
		err := os.WriteFile(testFilePath, testContent, 0644)
		require.NoError(t, err)

		jobID := uuid.New()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.Method == "POST" && r.URL.Path == "/v1/jobs":
				job := createTestJob(JobStatusCreated)
				job.Id = &jobID
				w.WriteHeader(http.StatusCreated)
				encodeJobResponse(w, job)

			case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data_size": 1024,
					"message":   "Upload successful",
				})

			case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
				job := createTestJob(JobStatusPending)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		job, err := client.CreateAndSubmitJobFromFile(ctx, "pandoc_md", testFilePath)

		require.NoError(t, err)
		require.NotNil(t, job)
		assert.Equal(t, jobID, *job.Id)
	})

	t.Run("file not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Server should not be called")
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		job, err := client.CreateAndSubmitJobFromFile(ctx, "pandoc_md", "/nonexistent/file.pdf")

		require.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "failed to open file")
	})
}

// TestProcessFile tests end-to-end file processing
func TestProcessFile(t *testing.T) {
	t.Run("successful file processing end-to-end", func(t *testing.T) {
		// Create temporary test file
		tmpDir := t.TempDir()
		testFilePath := filepath.Join(tmpDir, "test.pdf")
		testContent := []byte("PDF content simulation")
		err := os.WriteFile(testFilePath, testContent, 0644)
		require.NoError(t, err)

		jobID := uuid.New()
		outputData := []byte("# Converted Output")
		logsData := "Processing completed"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.Method == "POST" && r.URL.Path == "/v1/jobs":
				job := createTestJob(JobStatusCreated)
				job.Id = &jobID
				w.WriteHeader(http.StatusCreated)
				encodeJobResponse(w, job)

			case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
				// Verify multipart data
				err := r.ParseMultipartForm(10 << 20)
				require.NoError(t, err)
				file, _, err := r.FormFile("file")
				require.NoError(t, err)
				defer file.Close()
				uploadedContent, err := io.ReadAll(file)
				require.NoError(t, err)
				assert.Equal(t, testContent, uploadedContent)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data_size": 1024,
					"message":   "Upload successful",
				})

			case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
				job := createTestJob(JobStatusFinished)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/v1/jobs/") && !strings.Contains(r.URL.Path, "/output") && !strings.Contains(r.URL.Path, "/logs"):
				job := createTestJob(JobStatusFinished)
				job.Id = &jobID
				w.WriteHeader(http.StatusOK)
				encodeJobResponse(w, job)

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/output"):
				w.Header().Set("Content-Type", "text/markdown")
				w.WriteHeader(http.StatusOK)
				w.Write(outputData)

			case r.Method == "GET" && strings.Contains(r.URL.Path, "/logs"):
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(logsData))
			}
		}))
		defer server.Close()

		client, err := NewBsubClient(Config{
			APIKey:  "test-key",
			BaseURL: server.URL,
		})
		require.NoError(t, err)

		ctx := context.Background()
		result, err := client.ProcessFile(ctx, "pandoc_md", testFilePath)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.Equal(t, outputData, result.Output)
		assert.Equal(t, logsData, result.Logs)
	})
}

// TestJobStatus tests the job status enum
func TestJobStatus(t *testing.T) {
	statuses := []JobStatus{
		JobStatusCreated,
		JobStatusLoaded,
		JobStatusPending,
		JobStatusClaimed,
		JobStatusPreparing,
		JobStatusProcessing,
		JobStatusFinished,
		JobStatusFailed,
	}

	for _, status := range statuses {
		assert.NotEmpty(t, status, "Status should not be empty")
	}
}

// TestJobIsTerminal tests terminal state detection
func TestJobIsTerminal(t *testing.T) {
	tests := []struct {
		status     JobStatus
		isTerminal bool
	}{
		{JobStatusCreated, false},
		{JobStatusLoaded, false},
		{JobStatusPending, false},
		{JobStatusClaimed, false},
		{JobStatusPreparing, false},
		{JobStatusProcessing, false},
		{JobStatusFinished, true},
		{JobStatusFailed, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			isTerminal := tt.status == JobStatusFinished || tt.status == JobStatusFailed
			assert.Equal(t, tt.isTerminal, isTerminal)
		})
	}
}

// BenchmarkCreateAndSubmitJob benchmarks the job creation flow
func BenchmarkCreateAndSubmitJob(b *testing.B) {
	jobID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/jobs":
			job := createTestJob(JobStatusCreated)
			job.Id = &jobID
			w.WriteHeader(http.StatusCreated)
			encodeJobResponse(w, job)

		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/upload/"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data_size": 1024,
				"message":   "Upload successful",
			})

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/submit"):
			job := createTestJob(JobStatusPending)
			job.Id = &jobID
			w.WriteHeader(http.StatusOK)
			encodeJobResponse(w, job)
		}
	}))
	defer server.Close()

	client, err := NewBsubClient(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	data := []byte("test data content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := client.CreateAndSubmitJob(ctx, "pandoc_md", reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Example test demonstrating the complete workflow
func ExampleBsubClient_ProcessFile() {
	// This example shows how to use ProcessFile for end-to-end processing
	client, err := NewBsubClient(Config{
		APIKey: "your-api-key",
	})
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	ctx := context.Background()
	result, err := client.ProcessFile(ctx, "pandoc_md", "document.pdf")
	if err != nil {
		fmt.Printf("Processing failed: %v\n", err)
		return
	}

	fmt.Printf("Job completed with status: %s\n", *result.Job.Status)
	fmt.Printf("Output size: %d bytes\n", len(result.Output))
	fmt.Printf("Logs: %s\n", result.Logs)
}
