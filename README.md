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

Simple example:

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

    fmt.Printf("Job completed: %s\n", *result.Job.Id)
    fmt.Printf("Output: %s\n", string(result.Output))
}
```

For more examples, see [examples/](examples/) directory:
- [Basic usage](examples/basic/main.go) - Simple file processing
- [Batch processing](examples/batch/main.go) - Process multiple files concurrently
- [Custom workflow](examples/custom-workflow/main.go) - Step-by-step job control

You should be able to build them all by:

    make ex

Binaries will be in `bin/`.

## Development

You must have Go 1.24+ installed.
Most Linux/macOS distributions have this.
Go to https://go.dev/doc/install to learn more.

Then:

    make
    make test

This SDK is based on bsubio.io OpenAPI specification available at:

    https://app.bsub.io/static/openapi.yaml

You need OpenAPI compiler: https://github.com/oapi-codegen/oapi-codegen
Just install it with:

    make setup

To regenerate the code from public OpenAPI specs:

    make regen

## License

MIT

## Support

- Documentation: [https://docs.bsub.io](https://docs.bsub.io)
- Issues: [GitHub Issues](https://github.com/bsubio/bsubio-go/issues)
- Email: support@bsub.io
