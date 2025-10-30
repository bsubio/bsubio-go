package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/bsubio/bsubio-go"
)

func main() {
	// Get API key from environment variable
	apiKey := os.Getenv("BSUBIO_API_KEY")
	if apiKey == "" {
		log.Fatal("BSUBIO_API_KEY environment variable is required")
	}

	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <job-type> <file1> [file2] [file3] ...")
		fmt.Println("\nExample:")
		fmt.Println("  go run main.go pandoc_md *.pdf")
		os.Exit(1)
	}

	jobType := os.Args[1]
	files := os.Args[2:]

	// Create client
	client, err := bsubio.NewBsubClient(bsubio.Config{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	fmt.Printf("Processing %d files with job type: %s\n\n", len(files), jobType)

	// Process files concurrently
	var wg sync.WaitGroup
	results := make(chan processingResult, len(files))

	for _, filePath := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			result := processFile(ctx, client, jobType, file)
			results <- result
		}(filePath)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and display results
	successful := 0
	failed := 0

	for result := range results {
		if result.err != nil {
			fmt.Printf("[FAILED] %s: %v\n", result.fileName, result.err)
			failed++
		} else {
			fmt.Printf("[SUCCESS] %s: Job ID %s, Output: %d bytes\n",
				result.fileName,
				result.jobID,
				result.outputSize,
			)
			successful++

			// Optionally save output
			outputPath := result.fileName + ".out"
			if err := os.WriteFile(outputPath, result.output, 0644); err != nil {
				fmt.Printf("  Warning: Failed to save output to %s: %v\n", outputPath, err)
			} else {
				fmt.Printf("  Saved output to: %s\n", outputPath)
			}
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total files: %d\n", len(files))
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed: %d\n", failed)
}

type processingResult struct {
	fileName   string
	jobID      string
	output     []byte
	outputSize int
	err        error
}

func processFile(ctx context.Context, client *bsubio.BsubClient, jobType, filePath string) processingResult {
	fileName := filepath.Base(filePath)

	result := processingResult{
		fileName: fileName,
	}

	// Process the file
	jobResult, err := client.ProcessFile(ctx, jobType, filePath)
	if err != nil {
		result.err = err
		return result
	}

	result.jobID = jobResult.Job.Id.String()
	result.output = jobResult.Output
	result.outputSize = len(jobResult.Output)

	return result
}
