BINARY = triangled
CMD    = ./cmd/triangled

.PHONY: build test lint clean

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./internal/...

lint:
	golangci-lint run ./...

clean:
	go clean
	del /f $(BINARY).exe 2>nul || true
