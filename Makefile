.PHONY: build test clean example

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Go modules in this repository
MODULE_DIRS=. \
	examples/contract-events-processor \
	examples/contract-invocation-processor \
	examples/dual-mode-template \
	examples/postgresql-consumer

# Main package
MAIN_PACKAGE=./examples/basic_processor

# Example binary
EXAMPLE_BINARY=basic-processor

all: build

build:
	@set -e; for dir in $(MODULE_DIRS); do \
		echo "==> Building $$dir"; \
		(cd $$dir && $(GOBUILD) -v ./...); \
	done

test:
	@set -e; for dir in $(MODULE_DIRS); do \
		echo "==> Testing $$dir"; \
		(cd $$dir && $(GOTEST) -v ./...); \
	done

clean:
	@set -e; for dir in $(MODULE_DIRS); do \
		echo "==> Cleaning $$dir"; \
		(cd $$dir && $(GOCLEAN)); \
	done
	rm -f $(EXAMPLE_BINARY)

run-example:
	$(GOBUILD) -o $(EXAMPLE_BINARY) $(MAIN_PACKAGE)
	./$(EXAMPLE_BINARY)

tidy:
	@set -e; for dir in $(MODULE_DIRS); do \
		echo "==> Tidying $$dir"; \
		(cd $$dir && $(GOMOD) tidy); \
	done

vendor:
	$(GOMOD) vendor

update-proto:
	@echo "Updating proto dependencies..."
	$(GOGET) -u github.com/withObsrvr/flow-proto@latest
	$(GOMOD) tidy

install-tools:
	$(GOGET) -u google.golang.org/protobuf/cmd/protoc-gen-go
	$(GOGET) -u google.golang.org/grpc/cmd/protoc-gen-go-grpc

help:
	@echo "make build     - Build all modules"
	@echo "make test      - Test all modules"
	@echo "make clean     - Clean build artifacts"
	@echo "make run-example - Build and run the example processor"
	@echo "make tidy      - Tidy all modules"
	@echo "make vendor    - Vendor dependencies"
	@echo "make update-proto - Update proto dependencies"
	@echo "make install-tools - Install required development tools"
