.PHONY: build run clean test fmt vet

build:
	go build -o bin/silvia cmd/silvia/main.go

run:
	go run cmd/silvia/main.go

clean:
	rm -f silvia

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

all: fmt vet test build
