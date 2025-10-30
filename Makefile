.DEFAULT_GOAL := all

BUILD_DIR=$(CURDIR)/build/bin
COMMIT=$(shell git rev-parse HEAD)
DATE=$(shell git show -s --format=%cI HEAD)
TAG=$(shell git describe --tags --always --dirty)

LDFLAGS=-ldflags "-w -s -X 'main.gitCommit=$(COMMIT)' -X 'main.gitDate=$(DATE)' -X 'main.gitTag=$(TAG)'"

server:
	@echo "Building target: $@" 
	go build $(LDFLAGS) -o $(BUILD_DIR)/$@ $(CURDIR)/main.go
	@echo "Done building."

vscode:
	@echo "Building target: $@"
	cd ./web && ./build.sh --vscode $(BUILD_DIR)/dist
	@echo "Done building."

extensions:
	@echo "Building target: $@"
	cd ./web && ./build.sh --extensions $(BUILD_DIR)/dist
	@echo "Done building."

clean:
	@rm -rf $(BUILD_DIR)/*

all: server vscode extensions
