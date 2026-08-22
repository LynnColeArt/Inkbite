VERSION ?= dev
BINARY ?= inkbite
LDFLAGS := -X main.version=$(VERSION)
GOFILES := $(shell git ls-files '*.go')

.PHONY: build test acceptance vet fmt fmt-check quality ci dist clean

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/inkbite

test:
	go test ./...

acceptance:
	go test ./test/acceptance

vet:
	go vet ./...

fmt:
	gofmt -w $(GOFILES)

fmt-check:
	@test -z "$$(gofmt -l $(GOFILES))"

quality:
	./scripts/verify-ingestion-contract.sh quality

ci: quality

dist:
	./scripts/verify-ingestion-contract.sh package "$(VERSION)" "$(BINARY)" "$(DIST_DIR)"

clean:
	rm -rf bin dist coverage.out
