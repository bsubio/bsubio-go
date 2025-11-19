package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bsubio/bsubio-go"
)

func main() {
	// Get API key from environment variable
	apiKey := os.Getenv("BSUBIO_API_KEY")
	if apiKey == "" {
		log.Fatal("BSUBIO_API_KEY environment variable is required")
	}

	// Create client
	client, err := bsubio.NewBsubClient(bsubio.Config{
		APIKey: apiKey,
		// BaseURL: "http://localhost:9986", // Uncomment for local development
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Example 1: Get available processing types
	fmt.Println("Available processing types:")
	typesResp, err := client.GetTypesWithResponse(ctx)
	if err != nil {
		log.Fatalf("Failed to get types: %v", err)
	}

	if typesResp.JSON200 != nil && typesResp.JSON200.Types != nil {
		for _, procType := range *typesResp.JSON200.Types {
			fmt.Printf("  - %s: %s\n", *procType.Name, *procType.Description)
		}
	}
	fmt.Println()

	// Example 2: List recent jobs
	fmt.Println("Recent jobs:")
	limit := 5
	listResp, err := client.ListJobsWithResponse(ctx, &bsubio.ListJobsParams{
		Limit: &limit,
	})
	if err != nil {
		log.Fatalf("Failed to list jobs: %v", err)
	}

	if listResp.JSON200 != nil && listResp.JSON200.Data != nil && listResp.JSON200.Data.Jobs != nil {
		for _, job := range *listResp.JSON200.Data.Jobs {
			fmt.Printf("  Job %s: %s (type: %s)\n",
				*job.Id,
				*job.Status,
				*job.Type,
			)
		}
		if listResp.JSON200.Data.Total != nil {
			fmt.Printf("Total jobs: %d\n", *listResp.JSON200.Data.Total)
		}
	}
	fmt.Println()

	// Example 3: Process a file (if provided as argument)
	if len(os.Args) > 1 {
		filePath := os.Args[1]
		jobType := "test/linecount" // Simple test job for demo
		if len(os.Args) > 2 {
			jobType = os.Args[2]
		}

		fmt.Printf("Processing file: %s (type: %s)\n", filePath, jobType)

		result, err := client.ProcessFile(ctx, jobType, filePath)
		if err != nil {
			log.Fatalf("Failed to process file: %v", err)
		}

		fmt.Printf("Job completed successfully!\n")
		fmt.Printf("Job ID: %s\n", *result.Job.Id)
		if result.Job.DataSize != nil {
			fmt.Printf("Data size: %d bytes\n", *result.Job.DataSize)
		}
		fmt.Printf("Output size: %d bytes\n", len(result.Output))

		// Print first 200 chars of output
		if len(result.Output) > 0 {
			preview := string(result.Output)
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Printf("Output preview:\n%s\n", preview)
		}

		// Print logs if available
		if result.Logs != "" {
			fmt.Printf("\nLogs:\n%s\n", result.Logs)
		}
	} else {
		fmt.Println("To process a file, run:")
		fmt.Println("  go run main.go <file-path> [job-type]")
		fmt.Println("\nExample:")
		fmt.Println("  go run main.go document.pdf pandoc_md")
	}
}
