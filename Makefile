# Forest Runner Makefile
# Cruiser Note: Simplifying the cruise for the crew.

BINARY_NAME=forest-runner
MAIN_PACKAGE=./cmd/forest-runner

.PHONY: all build install clean test

all: build

build:
	go build -o $(BINARY_NAME) $(MAIN_PACKAGE)

install:
	go install $(MAIN_PACKAGE)

test:
	go test ./...

clean:
	rm -f $(BINARY_NAME)
	go clean
