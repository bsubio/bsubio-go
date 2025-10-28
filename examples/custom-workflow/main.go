package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bsubio/bsubio-go"
)

func main() {
	// Get API key from environment variable
	apiKey := os.Getenv("BSUBIO_API_KEY")
	if apiKey == "" {
		log.Fatal("BSUBIO_API_KEY environment variable is required")
	}

	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <job-type> <file-path>")
		fmt.Println("\nExample:")
		fmt.Println("  go run main.go pandoc_md document.pdf")
		os.Exit(1)
	}

	jobType := os.Args[1]
	filePath := os.Args[2]

	// Create client
	client, err := bsubio.NewBsubClient(bsubio.Config{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	fmt.Println("=== Custom Workflow Example ===")
	fmt.Printf("Job type: %s\n", jobType)
	fmt.Printf("File: %s\n\n", filePath)

	// Step 1: Create job
	fmt.Println("Step 1: Creating job...")
	createResp, err := client.CreateJobWithResponse(ctx, bsubio.CreateJobJSONRequestBody{
		Type: jobType,
	})
	if err != nil {
		log.Fatalf("Failed to create job: %v", err)
	}

	if createResp.JSON201 == nil || createResp.JSON201.Data == nil {
		log.Fatal("Unexpected response from create job")
	}

	job := createResp.JSON201.Data
	fmt.Printf("  Job created: %s\n", job.Id)
	fmt.Printf("  Status: %s\n", *job.Status)
	fmt.Printf("  Upload token: %s\n\n", *job.UploadToken)

	// Step 2: Upload file
	fmt.Println("Step 2: Uploading file...")
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	uploadResp, err := client.UploadJobDataWithBodyWithResponse(
		ctx,
		*job.Id,
		&bsubio.UploadJobDataParams{Token: *job.UploadToken},
		"application/octet-stream",
		file,
	)
	if err != nil {
		log.Fatalf("Failed to upload file: %v", err)
	}

	if uploadResp.JSON200 != nil {
		fmt.Printf("  Upload successful\n")
		if uploadResp.JSON200.DataSize != nil {
			fmt.Printf("  Data size: %d bytes\n\n", *uploadResp.JSON200.DataSize)
		}
	}

	// Step 3: Submit job for processing
	fmt.Println("Step 3: Submitting job for processing...")
	submitResp, err := client.SubmitJobWithResponse(ctx, *job.Id)
	if err != nil {
		log.Fatalf("Failed to submit job: %v", err)
	}

	if submitResp.JSON200 != nil && submitResp.JSON200.Message != nil {
		fmt.Printf("  %s\n\n", *submitResp.JSON200.Message)
	}

	// Step 4: Monitor job progress
	fmt.Println("Step 4: Monitoring job progress...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)
	var finishedJob *bsubio.Job

	for {
		select {
		case <-timeout:
			log.Fatal("Job timed out after 5 minutes")
		case <-ticker.C:
			jobResp, err := client.GetJobWithResponse(ctx, *job.Id)
			if err != nil {
				log.Printf("  Error checking status: %v", err)
				continue
			}

			if jobResp.JSON200 == nil || jobResp.JSON200.Data == nil {
				continue
			}

			currentJob := jobResp.JSON200.Data
			fmt.Printf("  Status: %s", *currentJob.Status)

			if currentJob.ClaimedBy != nil {
				fmt.Printf(" (claimed by: %s)", *currentJob.ClaimedBy)
			}
			fmt.Println()

			// Check if job is finished
			if *currentJob.Status == bsubio.JobStatusFinished || *currentJob.Status == bsubio.JobStatusFailed {
				finishedJob = currentJob
				goto done
			}
		}
	}

done:
	fmt.Println()

	// Step 5: Retrieve results
	if finishedJob.Status != nil && *finishedJob.Status == bsubio.JobStatusFailed {
		fmt.Println("Step 5: Job failed!")
		if finishedJob.ErrorCode != nil {
			fmt.Printf("  Error code: %s\n", *finishedJob.ErrorCode)
		}
		if finishedJob.ErrorMessage != nil {
			fmt.Printf("  Error message: %s\n", *finishedJob.ErrorMessage)
		}

		// Still try to get logs
		logsResp, err := client.GetJobLogs(ctx, *job.Id)
		if err == nil {
			defer logsResp.Body.Close()
			fmt.Println("\n  Logs:")
			// Could read and display logs here
		}

		os.Exit(1)
	}

	fmt.Println("Step 5: Retrieving results...")

	// Get output
	outputResp, err := client.GetJobOutput(ctx, *job.Id)
	if err != nil {
		log.Fatalf("Failed to get output: %v", err)
	}
	defer outputResp.Body.Close()

	output, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read output: %v", err)
	}

	fmt.Printf("  Output size: %d bytes\n", len(output))

	// Save output
	outputPath := "output.result"
	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		log.Printf("  Warning: Failed to save output: %v", err)
	} else {
		fmt.Printf("  Saved output to: %s\n", outputPath)
	}

	// Get logs
	logsResp, err := client.GetJobLogs(ctx, *job.Id)
	if err == nil {
		defer logsResp.Body.Close()
		fmt.Println("  Logs retrieved")
	}

	fmt.Println("\n=== Job completed successfully! ===")
	if finishedJob.CreatedAt != nil && finishedJob.FinishedAt != nil {
		duration := finishedJob.FinishedAt.Sub(*finishedJob.CreatedAt)
		fmt.Printf("Total processing time: %s\n", duration.Round(time.Second))
	}
}
