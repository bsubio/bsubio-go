all: build

build:
	go build

regen:
	go tool oapi-codegen -config .oapi-codegen.yaml https://app.bsub.io/static/openapi.yaml 

test:
	go test ./...

setup:
	go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
