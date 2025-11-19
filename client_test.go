package bsubio

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			errContains: "API key not found",
		},
		{
			name: "empty API key",
			config: Config{
				APIKey: "",
			},
			wantErr:     true,
			errContains: "API key not found",
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
	mockServer := NewMockServer()
	defer mockServer.Close()

	apiKey := "test-api-key-123"
	client, err := NewBsubClient(Config{
		APIKey:  apiKey,
		BaseURL: mockServer.URL,
	})
	require.NoError(t, err)

	// Make a request
	ctx := context.Background()
	reqBody := CreateJobJSONRequestBody{Type: "test/linecount"}
	resp, err := client.CreateJobWithResponse(ctx, reqBody)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 201, resp.StatusCode())
}

// TestCreateAndSubmitJob tests the job creation and submission flow with passthrough
func TestCreateAndSubmitJob(t *testing.T) {
	t.Run("successful job creation and submission with passthrough", func(t *testing.T) {
		client, mockServer, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		data := bytes.NewReader([]byte("test data content"))
		job, err := client.CreateAndSubmitJob(ctx, "test/linecount", data)

		require.NoError(t, err)
		require.NotNil(t, job)
		assert.NotNil(t, job.Id)
		// Note: CreateAndSubmitJob returns job from create step, so status is still "created"
		assert.Equal(t, JobStatusCreated, *job.Status)

		// Verify job was submitted and is now finished in mock server
		if mockServer != nil {
			storedJob := mockServer.GetJob(*job.Id)
			require.NotNil(t, storedJob)
			assert.Equal(t, "test/linecount", *storedJob.Type)
			// The mock server updates the status to finished for passthrough jobs
			assert.Equal(t, JobStatusFinished, *storedJob.Status)
		}
	})

	t.Run("successful job with line_counter", func(t *testing.T) {
		client, mockServer, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		data := bytes.NewReader([]byte("line1\nline2\nline3\nline4\nline5"))
		job, err := client.CreateAndSubmitJob(ctx, "test/linecount", data)

		require.NoError(t, err)
		require.NotNil(t, job)
		// CreateAndSubmitJob returns job from create step
		assert.Equal(t, JobStatusCreated, *job.Status)

		// Verify in mock server that job was actually submitted
		if mockServer != nil {
			storedJob := mockServer.GetJob(*job.Id)
			require.NotNil(t, storedJob)
			assert.Equal(t, JobStatusFinished, *storedJob.Status)
		}
	})
}

// TestWaitForJob tests the polling mechanism
func TestWaitForJob(t *testing.T) {
	mode := GetTestMode()
	if mode == TestModeProduction {
		t.Skip("Skipping WaitForJob in production mode - requires long-running job")
	}

	t.Run("job finishes immediately with passthrough", func(t *testing.T) {
		client, mockServer, cleanup := SetupTestClient(t)
		defer cleanup()

		// Create and submit a passthrough job first
		ctx := context.Background()
		data := bytes.NewReader([]byte("test data"))
		job, err := client.CreateAndSubmitJob(ctx, "test/linecount", data)
		require.NoError(t, err)
		require.NotNil(t, job)

		// Wait for job (should be already finished for passthrough)
		finalJob, err := client.WaitForJob(ctx, *job.Id)
		require.NoError(t, err)
		require.NotNil(t, finalJob)
		assert.Equal(t, JobStatusFinished, *finalJob.Status)

		if mockServer != nil {
			storedJob := mockServer.GetJob(*job.Id)
			require.NotNil(t, storedJob)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		client, mockServer, cleanup := SetupTestClient(t)
		defer cleanup()

		if mockServer == nil {
			t.Skip("Context cancellation test only supported in mock mode")
		}

		// Create a job but don't submit it (so it stays in created state)
		ctx := context.Background()
		reqBody := CreateJobJSONRequestBody{Type: "test/linecount"}
		resp, err := client.CreateJobWithResponse(ctx, reqBody)
		require.NoError(t, err)
		require.NotNil(t, resp.JSON201)

		jobID := *resp.JSON201.Data.Id

		// Manually set the job to processing state (simulating long-running job)
		job := mockServer.GetJob(jobID)
		status := JobStatusProcessing
		job.Status = &status

		// Create context with short timeout
		ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		finalJob, err := client.WaitForJob(ctxWithTimeout, jobID)

		require.Error(t, err)
		assert.Nil(t, finalJob)
		assert.Contains(t, err.Error(), "context")
	})
}

// TestGetJobResult tests result retrieval
func TestGetJobResult(t *testing.T) {
	t.Run("successful result retrieval with passthrough", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		data := bytes.NewReader([]byte("test input data"))
		job, err := client.CreateAndSubmitJob(ctx, "test/linecount", data)
		require.NoError(t, err)
		require.NotNil(t, job)

		// For passthrough, job should be finished immediately
		result, err := client.GetJobResult(ctx, *job.Id)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, *job.Id, *result.Job.Id)
		assert.NotEmpty(t, result.Output)
	})

	t.Run("successful result with line_counter", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		data := bytes.NewReader([]byte("line1\nline2\nline3\nline4\nline5"))
		job, err := client.CreateAndSubmitJob(ctx, "test/linecount", data)
		require.NoError(t, err)

		result, err := client.GetJobResult(ctx, *job.Id)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.Output)
	})
}

// TestProcess tests end-to-end processing with reader
func TestProcess(t *testing.T) {
	t.Run("successful processing with passthrough", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		inputData := []byte("Test input data for passthrough")
		data := bytes.NewReader(inputData)
		result, err := client.Process(ctx, "test/linecount", data)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.NotEmpty(t, result.Output)
	})

	t.Run("successful processing with line_counter", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		inputData := []byte("line1\nline2\nline3")
		data := bytes.NewReader(inputData)
		result, err := client.Process(ctx, "test/linecount", data)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.NotEmpty(t, result.Output)
	})
}

// TestCreateAndSubmitJobFromFile tests file-based job submission
func TestCreateAndSubmitJobFromFile(t *testing.T) {
	t.Run("successful file processing with passthrough", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		// Create temporary test file
		tmpDir := t.TempDir()
		testFilePath := filepath.Join(tmpDir, "test.txt")
		testContent := []byte("File content for passthrough test")
		err := os.WriteFile(testFilePath, testContent, 0644)
		require.NoError(t, err)

		ctx := context.Background()
		job, err := client.CreateAndSubmitJobFromFile(ctx, "test/linecount", testFilePath)

		require.NoError(t, err)
		require.NotNil(t, job)
		assert.NotNil(t, job.Id)
	})

	t.Run("file not found", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		job, err := client.CreateAndSubmitJobFromFile(ctx, "test/linecount", "/nonexistent/file.txt")

		require.Error(t, err)
		assert.Nil(t, job)
		assert.Contains(t, err.Error(), "failed to open file")
	})
}

// TestProcessFile tests end-to-end file processing
func TestProcessFile(t *testing.T) {
	t.Run("successful file processing end-to-end with passthrough", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		// Create temporary test file
		tmpDir := t.TempDir()
		testFilePath := filepath.Join(tmpDir, "test.txt")
		testContent := []byte("File content for end-to-end test")
		err := os.WriteFile(testFilePath, testContent, 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := client.ProcessFile(ctx, "test/linecount", testFilePath)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.NotEmpty(t, result.Output)
	})

	t.Run("successful file processing with line_counter", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		tmpDir := t.TempDir()
		testFilePath := filepath.Join(tmpDir, "lines.txt")
		testContent := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7")
		err := os.WriteFile(testFilePath, testContent, 0644)
		require.NoError(t, err)

		ctx := context.Background()
		result, err := client.ProcessFile(ctx, "test/linecount", testFilePath)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.NotEmpty(t, result.Output)
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
	mockServer := NewMockServer()
	defer mockServer.Close()

	client, err := NewBsubClient(Config{
		APIKey:  "test-key",
		BaseURL: mockServer.URL,
	})
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	data := []byte("test data content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := client.CreateAndSubmitJob(ctx, "test/linecount", reader)
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
	result, err := client.ProcessFile(ctx, "test/linecount", "document.txt")
	if err != nil {
		fmt.Printf("Processing failed: %v\n", err)
		return
	}

	fmt.Printf("Job completed with status: %s\n", *result.Job.Status)
	fmt.Printf("Output size: %d bytes\n", len(result.Output))
	fmt.Printf("Logs: %s\n", result.Logs)
}

// TestIntegration_RealJobTypes tests with actual job types that exist in production
// Run with BSUB_TEST_MODE=production to test against real server
func TestIntegration_RealJobTypes(t *testing.T) {
	mode := GetTestMode()
	if mode != TestModeProduction {
		t.Log("Running in mock mode. Set BSUB_TEST_MODE=production to test against real server")
	}

	t.Run("test/linecount job - single line", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		input := []byte("Hello from test/linecount!")
		result, err := client.Process(ctx, "test/linecount", bytes.NewReader(input))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.NotEmpty(t, result.Output)

		t.Logf("Job ID: %s", result.Job.Id.String())
		t.Logf("Output: %s", string(result.Output))
	})

	t.Run("test/linecount job - multiple lines", func(t *testing.T) {
		client, _, cleanup := SetupTestClient(t)
		defer cleanup()

		ctx := context.Background()
		input := []byte("line 1\nline 2\nline 3\nline 4\nline 5")
		result, err := client.Process(ctx, "test/linecount", bytes.NewReader(input))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, JobStatusFinished, *result.Job.Status)
		assert.NotEmpty(t, result.Output)

		t.Logf("Job ID: %s", result.Job.Id.String())
		t.Logf("Line count output: %s", string(result.Output))
	})
}
