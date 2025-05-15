.PHONY: build test clean example

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Main package
MAIN_PACKAGE=./examples/basic_processor

# Example binary
EXAMPLE_BINARY=basic-processor

all: build

build:
	$(GOBUILD) -v ./...

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(EXAMPLE_BINARY)

run-example:
	$(GOBUILD) -o $(EXAMPLE_BINARY) $(MAIN_PACKAGE)
	./$(EXAMPLE_BINARY)

tidy:
	$(GOMOD) tidy

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
	@echo "make build     - Build the SDK"
	@echo "make test      - Run tests"
	@echo "make clean     - Clean build artifacts"
	@echo "make run-example - Build and run the example processor"
	@echo "make tidy      - Tidy up the go.mod file"
	@echo "make vendor    - Vendor dependencies"
	@echo "make update-proto - Update proto dependencies"
	@echo "make install-tools - Install required development tools"