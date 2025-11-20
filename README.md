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

Our CLI will create you `~/.config/bsubio/config.json` automatically.
Visit https://www.bsub.io and follow the installation steps to get `bsubio` to work for you.
Then `bsubio register` should give you new account with API key created.

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
    config := bsubio.LoadConfig()

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

## Testing

The SDK includes a comprehensive test suite that supports two testing modes:

### Mock Tests (Default)

Run tests against an in-memory mock server (fast and deterministic):

```bash
go test -v -cover
```

### Production Tests

Run tests against the real bsub.io API to verify integration:

```bash
# Using environment variable
export BSUB_TEST_MODE=production
export BSUB_API_KEY=your_api_key_here
go test -v

# Or using config file (~/.config/bsubio/config.json)
export BSUB_TEST_MODE=production
go test -v
```

Production tests verify that the SDK works correctly with the actual API and are useful for catching API changes.

## License

MIT

## Support

- Documentation: [https://docs.bsub.io](https://docs.bsub.io)
- Issues: [GitHub Issues](https://github.com/bsubio/bsubio-go/issues)
- Email: support@bsub.io
