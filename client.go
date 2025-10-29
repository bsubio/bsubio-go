package bsubio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// BsubClient wraps the generated API client with helper methods
type BsubClient struct {
	*ClientWithResponses
	apiKey string
}

// Config holds configuration for the BSUB.IO client
type Config struct {
	// APIKey is your BSUB.IO API key
	APIKey string
	// BaseURL is the API server URL (defaults to production)
	BaseURL string
	// HTTPClient is optional custom HTTP client
	HTTPClient *http.Client
}

// NewBsubClient creates a new BSUB.IO API client
func NewBsubClient(config Config) (*BsubClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://app.bsub.io"
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Create client with auth interceptor
	clientWithResponses, err := NewClientWithResponses(
		baseURL,
		WithHTTPClient(httpClient),
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+config.APIKey)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &BsubClient{
		ClientWithResponses: clientWithResponses,
		apiKey:              config.APIKey,
	}, nil
}

// JobResult represents the result of a completed job
type JobResult struct {
	Job    *Job
	Output []byte
	Logs   string
}

// CreateAndSubmitJob is a helper that creates a job, uploads data, and submits it for processing
func (c *BsubClient) CreateAndSubmitJob(ctx context.Context, jobType string, data io.Reader) (*Job, error) {
	// Create job
	createResp, err := c.CreateJobWithResponse(ctx, CreateJobJSONRequestBody{
		Type: jobType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	if createResp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("failed to create job: status %d", createResp.StatusCode())
	}

	if createResp.JSON201 == nil || createResp.JSON201.Data == nil {
		return nil, fmt.Errorf("unexpected response format")
	}

	job := createResp.JSON201.Data
	if job.UploadToken == nil {
		return nil, fmt.Errorf("no upload token in response")
	}

	// Upload data as multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "upload")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, data); err != nil {
		return nil, fmt.Errorf("failed to copy data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	uploadResp, err := c.UploadJobDataWithBodyWithResponse(ctx, *job.Id, &UploadJobDataParams{
		Token: *job.UploadToken,
	}, writer.FormDataContentType(), &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to upload data: %w", err)
	}

	if uploadResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to upload data: status %d", uploadResp.StatusCode())
	}

	// Submit job
	submitResp, err := c.SubmitJobWithResponse(ctx, *job.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	if submitResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to submit job: status %d", submitResp.StatusCode())
	}

	return job, nil
}

// CreateAndSubmitJobFromFile is a helper that creates a job, uploads a file, and submits it for processing
func (c *BsubClient) CreateAndSubmitJobFromFile(ctx context.Context, jobType string, filePath string) (*Job, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return c.CreateAndSubmitJob(ctx, jobType, file)
}

// WaitForJob polls the job status until it's finished or failed
func (c *BsubClient) WaitForJob(ctx context.Context, jobID JobId) (*Job, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := c.GetJobWithResponse(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("failed to get job status: %w", err)
		}

		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("failed to get job status: status %d", resp.StatusCode())
		}

		if resp.JSON200 == nil || resp.JSON200.Data == nil {
			return nil, fmt.Errorf("unexpected response format")
		}

		job := resp.JSON200.Data

		// Check if job is in a terminal state
		if job.Status != nil && (*job.Status == JobStatusFinished || *job.Status == JobStatusFailed) {
			return job, nil
		}

		// Wait before polling again (simple implementation, could be improved with backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
			// Continue polling
		}
	}
}

// GetJobResult retrieves the complete result of a finished job including output and logs
func (c *BsubClient) GetJobResult(ctx context.Context, jobID JobId) (*JobResult, error) {
	// Get job details
	jobResp, err := c.GetJobWithResponse(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if jobResp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get job: status %d", jobResp.StatusCode())
	}

	if jobResp.JSON200 == nil || jobResp.JSON200.Data == nil {
		return nil, fmt.Errorf("unexpected response format")
	}

	job := jobResp.JSON200.Data

	result := &JobResult{
		Job: job,
	}

	// Get output if job is finished
	if job.Status != nil && *job.Status == JobStatusFinished {
		outputResp, err := c.GetJobOutput(ctx, jobID)
		if err != nil {
			return nil, fmt.Errorf("failed to get job output: %w", err)
		}
		defer outputResp.Body.Close()

		if outputResp.StatusCode == http.StatusOK {
			output, err := io.ReadAll(outputResp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read output: %w", err)
			}
			result.Output = output
		}
	}

	// Get logs
	logsResp, err := c.GetJobLogs(ctx, jobID)
	if err != nil {
		// Logs might not always be available, so we don't fail here
		return result, nil
	}
	defer logsResp.Body.Close()

	if logsResp.StatusCode == http.StatusOK {
		logs, err := io.ReadAll(logsResp.Body)
		if err != nil {
			return result, nil
		}
		result.Logs = string(logs)
	}

	return result, nil
}

// ProcessFile is a complete helper that creates, uploads, submits, waits, and retrieves results
func (c *BsubClient) ProcessFile(ctx context.Context, jobType string, filePath string) (*JobResult, error) {
	// Create and submit job
	job, err := c.CreateAndSubmitJobFromFile(ctx, jobType, filePath)
	if err != nil {
		return nil, err
	}

	// Wait for completion
	finishedJob, err := c.WaitForJob(ctx, *job.Id)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for job: %w", err)
	}

	// Check if job failed
	if finishedJob.Status != nil && *finishedJob.Status == JobStatusFailed {
		result, _ := c.GetJobResult(ctx, *job.Id)
		if result != nil && finishedJob.ErrorMessage != nil {
			return result, fmt.Errorf("job failed: %s", *finishedJob.ErrorMessage)
		}
		return result, fmt.Errorf("job failed")
	}

	// Get results
	return c.GetJobResult(ctx, *job.Id)
}

// Process is a complete helper that creates, uploads, submits, waits, and retrieves results from a reader
func (c *BsubClient) Process(ctx context.Context, jobType string, data io.Reader) (*JobResult, error) {
	// Create and submit job
	job, err := c.CreateAndSubmitJob(ctx, jobType, data)
	if err != nil {
		return nil, err
	}

	// Wait for completion
	finishedJob, err := c.WaitForJob(ctx, *job.Id)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for job: %w", err)
	}

	// Check if job failed
	if finishedJob.Status != nil && *finishedJob.Status == JobStatusFailed {
		result, _ := c.GetJobResult(ctx, *job.Id)
		if result != nil && finishedJob.ErrorMessage != nil {
			return result, fmt.Errorf("job failed: %s", *finishedJob.ErrorMessage)
		}
		return result, fmt.Errorf("job failed")
	}

	// Get results
	return c.GetJobResult(ctx, *job.Id)
}
