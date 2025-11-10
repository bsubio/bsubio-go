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
