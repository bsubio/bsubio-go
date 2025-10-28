# BSUB.IO Go SDK

Official Go SDK for the [BSUB.IO](https://bsub.io) API - Batch processing for compute-intensive workloads.

## Features

- **Simple API**: Easy-to-use client with helper methods for common workflows
- **Type-safe**: Generated from OpenAPI spec with full type safety
- **Complete**: Access to all BSUB.IO API endpoints
- **Flexible**: Use high-level helpers or low-level API methods

## Installation

```bash
go get github.com/bsubio/bsubio-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bsubio/bsubio-go"
)

func main() {
    // Create client
    client, err := bsubio.NewBsubClient(bsubio.Config{
        APIKey: "your-api-key",
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Process a file (one-liner)
    result, err := client.ProcessFile(ctx, "pandoc_md", "document.pdf")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Job completed: %s\n", result.Job.Id)
    fmt.Printf("Output: %s\n", string(result.Output))
}
```

## Configuration

### Creating a Client

```go
client, err := bsubio.NewBsubClient(bsubio.Config{
    APIKey:  "your-api-key",           // Required
    BaseURL: "https://app.bsub.io",     // Optional, defaults to production
    HTTPClient: &http.Client{          // Optional, defaults to http.DefaultClient
        Timeout: 30 * time.Second,
    },
})
```

### Using Custom Server

For local development or custom deployments:

```go
client, err := bsubio.NewBsubClient(bsubio.Config{
    APIKey:  "your-api-key",
    BaseURL: "http://localhost:9986",
})
```

## Usage Examples

### High-Level Helpers

The SDK provides convenient helper methods for common workflows:

#### Process a File (Complete Workflow)

```go
// Automatically creates job, uploads file, submits, waits for completion
result, err := client.ProcessFile(ctx, "pandoc_md", "document.pdf")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Output: %s\n", string(result.Output))
fmt.Printf("Logs: %s\n", result.Logs)
```

#### Process from Reader

```go
file, _ := os.Open("document.pdf")
defer file.Close()

result, err := client.Process(ctx, "pandoc_md", file)
if err != nil {
    log.Fatal(err)
}

// Save output
os.WriteFile("output.md", result.Output, 0644)
```

### Step-by-Step Workflow

For more control, use the individual methods:

```go
// 1. Create job
createResp, err := client.CreateJobWithResponse(ctx, bsubio.CreateJobJSONRequestBody{
    Type: "pandoc_md",
})
if err != nil {
    log.Fatal(err)
}
job := createResp.JSON201.Data

// 2. Upload data
file, _ := os.Open("document.pdf")
defer file.Close()

uploadResp, err := client.UploadJobDataWithBodyWithResponse(
    ctx,
    *job.Id,
    &bsubio.UploadJobDataParams{Token: *job.UploadToken},
    "application/octet-stream",
    file,
)

// 3. Submit job for processing
submitResp, err := client.SubmitJobWithResponse(ctx, *job.Id)

// 4. Wait for completion
finishedJob, err := client.WaitForJob(ctx, *job.Id)

// 5. Get results
result, err := client.GetJobResult(ctx, *job.Id)
```

### List Jobs

```go
// List all jobs
resp, err := client.ListJobsWithResponse(ctx, &bsubio.ListJobsParams{})
if err != nil {
    log.Fatal(err)
}

for _, job := range resp.JSON200.Data.Jobs {
    fmt.Printf("Job %s: %s\n", job.Id, *job.Status)
}

// Filter by status
status := bsubio.ListJobsParamsStatusFinished
limit := 10
resp, err = client.ListJobsWithResponse(ctx, &bsubio.ListJobsParams{
    Status: &status,
    Limit:  &limit,
})
```

### Get Job Status

```go
resp, err := client.GetJobWithResponse(ctx, jobID)
if err != nil {
    log.Fatal(err)
}

job := resp.JSON200.Data
fmt.Printf("Status: %s\n", *job.Status)
if job.ErrorMessage != nil {
    fmt.Printf("Error: %s\n", *job.ErrorMessage)
}
```

### Get Job Output

```go
outputResp, err := client.GetJobOutput(ctx, jobID)
if err != nil {
    log.Fatal(err)
}
defer outputResp.Body.Close()

output, _ := io.ReadAll(outputResp.Body)
fmt.Println(string(output))
```

### Get Job Logs

```go
logsResp, err := client.GetJobLogs(ctx, jobID)
if err != nil {
    log.Fatal(err)
}
defer logsResp.Body.Close()

logs, _ := io.ReadAll(logsResp.Body)
fmt.Println(string(logs))
```

### Cancel a Job

```go
resp, err := client.CancelJobWithResponse(ctx, jobID)
if err != nil {
    log.Fatal(err)
}
fmt.Println(resp.JSON200.Message)
```

### Delete a Job

```go
resp, err := client.DeleteJobWithResponse(ctx, jobID)
if err != nil {
    log.Fatal(err)
}
// Status code 204 means success
```

### Get Available Processing Types

```go
resp, err := client.GetTypesWithResponse(ctx)
if err != nil {
    log.Fatal(err)
}

for _, procType := range resp.JSON200.Types {
    fmt.Printf("%s: %s\n", *procType.Name, *procType.Description)
}
```

## Processing Types

Common processing types available:

- `passthrough` - Pass data through unchanged
- `json_format` - Format JSON data
- `pandoc_md` - Convert document to Markdown
- `pandoc_pdf` - Convert document to PDF
- `pandoc_html` - Convert document to HTML
- `ffprobe` - Extract media metadata
- `ffmpeg_mp4` - Convert video to MP4
- `imagemagick_resize` - Resize image
- `imagemagick_thumbnail` - Generate thumbnail
- `pdf_text` - Extract text from PDF
- `ocr_tesseract` - OCR image to text
- `wkhtmltopdf` - HTML to PDF conversion

Use `GetTypes()` to get the current list of available types from the server.

## Job States

Jobs progress through these states:

- `Created` - Job created, awaiting data upload
- `Loaded` - Data uploaded successfully
- `Pending` - Waiting in queue for a worker
- `Claimed` - Worker claimed the job
- `Preparing` - Worker preparing to process
- `InProgress` - Processing started
- `Finished` - Processing completed successfully
- `Failed` - Processing failed with error

## Error Handling

```go
result, err := client.ProcessFile(ctx, "pandoc_md", "document.pdf")
if err != nil {
    log.Printf("Error processing file: %v\n", err)
    return
}

// Check if job failed during processing
if result.Job.Status != nil && *result.Job.Status == bsubio.JobStatusFailed {
    if result.Job.ErrorMessage != nil {
        log.Printf("Job failed: %s\n", *result.Job.ErrorMessage)
    }
    if result.Job.ErrorCode != nil {
        log.Printf("Error code: %s\n", *result.Job.ErrorCode)
    }
}
```

## Context and Timeouts

Use context for timeouts and cancellation:

```go
// Set a timeout for the entire operation
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result, err := client.ProcessFile(ctx, "pandoc_md", "large-document.pdf")
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Processing timed out")
    }
    return
}
```

## API Reference

The SDK provides two levels of access:

### High-Level Methods (BsubClient)

- `ProcessFile(ctx, jobType, filePath)` - Complete workflow from file
- `Process(ctx, jobType, reader)` - Complete workflow from reader
- `CreateAndSubmitJob(ctx, jobType, reader)` - Create, upload, and submit
- `CreateAndSubmitJobFromFile(ctx, jobType, filePath)` - Same but from file
- `WaitForJob(ctx, jobID)` - Poll until job finishes
- `GetJobResult(ctx, jobID)` - Get complete job result with output and logs

### Low-Level Methods (Generated)

All OpenAPI operations are available with `*WithResponse` suffix:

- `CreateJobWithResponse(ctx, body)`
- `ListJobsWithResponse(ctx, params)`
- `GetJobWithResponse(ctx, jobID)`
- `DeleteJobWithResponse(ctx, jobID)`
- `SubmitJobWithResponse(ctx, jobID)`
- `CancelJobWithResponse(ctx, jobID)`
- `GetJobOutput(ctx, jobID)`
- `GetJobLogs(ctx, jobID)`
- `UploadJobDataWithBodyWithResponse(ctx, jobID, params, contentType, body)`
- `GetTypesWithResponse(ctx)`
- `GetVersionWithResponse(ctx)`

## Examples

See the [examples](examples/) directory for complete working examples:

- [Basic usage](examples/basic/main.go)
- [Batch processing](examples/batch/main.go)
- [Error handling](examples/error-handling/main.go)
- [Custom workflow](examples/custom-workflow/main.go)

## Development

### Building

```bash
go build
```

### Testing

```bash
go test ./...
```

### Regenerating Code

If the OpenAPI spec changes:

```bash
oapi-codegen -config .oapi-codegen.yaml openapi.yaml
```

## License

MIT

## Support

- Documentation: [https://docs.bsub.io](https://docs.bsub.io)
- Issues: [GitHub Issues](https://github.com/bsubio/bsubio-go/issues)
- Email: support@bsub.io
