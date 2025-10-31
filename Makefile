all: build

build:
	go build

regen:
	go tool oapi-codegen -config ./.oapi-codegen.yaml https://app.bsub.io/static/openapi.yaml 

test:
	go test ./...

setup:
	go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0

ex:
	mkdir -p bin/
	go build -o bin/example-comprehensive examples/comprehensive/main.go
	go build -o bin/basic examples/basic/main.go
	go build -o bin/batch examples/batch/main.go
	go build -o bin/custom-workflow examples/custom-workflow/main.go
