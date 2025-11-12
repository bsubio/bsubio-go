# Go SDK for bsub.io (BETA)

[we're early project; anything here can change withtout a warning until we push v1]

Official Golang SDK for the [BSUB.IO](https://bsub.io) API.

You can use it to build Golang apps for data processing.

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

### Getting an API Key

You have two options to get your API key:

1. **Register via CLI** (recommended): Run `bsubio register` to create an account and automatically configure your credentials
2. **Register via Web**: Visit [https://bsub.io](https://bsub.io) to create an account and get your API key

### Simple Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/bsubio/bsubio-go"
)

func main() {
    // Load configuration (reads from ~/.config/bsubio/config.json or BSUBIO_API_KEY env var)
    config, err := bsubio.LoadConfig()
    if err != nil {
        log.Fatal(err)
    }

    // Create client
    client, err := bsubio.NewBsubClient(config)
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
You need OpenAPI compiler: https://github.com/oapi-codegen/oapi-codegen

Then:

    make
    make test

This SDK is based on bsubio.io OpenAPI specification available at:

    https://app.bsub.io/static/openapi.yaml

## License

MIT

## Support

- Documentation: [https://docs.bsub.io](https://docs.bsub.io)
- Issues: [GitHub Issues](https://github.com/bsubio/bsubio-go/issues)
- Email: support@bsub.io
