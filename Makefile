.PHONY: build install test vet clean

build:
	go build -o dotllm .

install:
	go build -o $(shell go env GOPATH)/bin/dotllm .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f dotllm
