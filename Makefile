.PHONY: build test lint fmt clean release-snapshot tidy

BIN_DIR := bin
LDFLAGS := -s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/deepwork ./cmd/deepwork
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/deepwork-apply ./cmd/deepwork-apply

test:
	go test -race ./...

lint:
	go vet ./...
	@fmt_out=$$(gofmt -l .); if [ -n "$$fmt_out" ]; then echo "gofmt issues:"; echo "$$fmt_out"; exit 1; fi

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)

release-snapshot:
	goreleaser release --snapshot --clean
