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

	// Check if file path is provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <file-path>")
		fmt.Println("\nExample:")
		fmt.Println("  go run main.go document.pdf")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Create client
	client, err := bsubio.NewBsubClient(bsubio.Config{
		APIKey: apiKey,
		// BaseURL: "http://localhost:9986", // Uncomment for local development
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Process file with pandoc_md type
	fmt.Printf("Processing file: %s\n", filePath)

	result, err := client.ProcessFile(ctx, "pandoc_md", filePath)
	if err != nil {
		log.Fatalf("Failed to process file: %v", err)
	}

	// Display results
	fmt.Printf("\nJob completed successfully!\n")
	fmt.Printf("Job ID: %s\n", *result.Job.Id)
	if result.Job.DataSize != nil {
		fmt.Printf("Input size: %d bytes\n", *result.Job.DataSize)
	}
	fmt.Printf("Output size: %d bytes\n", len(result.Output))

	// Print first 500 chars of markdown output
	if len(result.Output) > 0 {
		preview := string(result.Output)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Printf("\nMarkdown output:\n%s\n", preview)
	}

	// Print logs if available
	if result.Logs != "" {
		fmt.Printf("\nLogs:\n%s\n", result.Logs)
	}
}
