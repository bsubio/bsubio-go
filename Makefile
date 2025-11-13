all: build

build:
	go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0
	go tool oapi-codegen -config ./.oapi-codegen.yaml ../app.bsub.io/static/openapi.yaml
	go build

test:
	go test ./...

ex:
	mkdir -p bin/
	go build -o bin/example-comprehensive examples/comprehensive/main.go
	go build -o bin/basic examples/basic/main.go
	go build -o bin/batch examples/batch/main.go
	go build -o bin/custom-workflow examples/custom-workflow/main.go

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	which golangci-lint > /dev/null || (echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

check: fmt vet lint test
