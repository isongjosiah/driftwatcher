COUNTERFEITER      := $(shell command -v counterfeiter 2> /dev/null)
STRINGER := $(shell command -v stringer 2> /dev/null)
TEST_FUNCTION ?= TestMySpecificFunction
PACKAGE_PATH  ?= ./path/to/package

# Check if Docker daemon is running and accessible
check/docker:
	@echo "Checking Docker daemon status..."
	@docker info > /dev/null 2>&1 || (echo "Docker daemon is not running or not accessible. Please start Docker Desktop." && exit 1)
	@echo "Docker daemon is running."

pull/localstack: check/docker
	@echo "Attempting to pull localstack/localstack:latest..."
	docker pull localstack/localstack:latest
	@echo "LocalStack image pulled successfully."


get/stringer:
ifndef STRINGER
	@echo "installing stringer"
	@go get -u -a golang.org/x/tools/cmd/stringer
endif

get/counterfeiter:
ifndef COUNTERFEITER
	@echo "installing counterfeiter"
	@go get -u github.com/maxbrunsfeld/counterfeiter/v6
endif

generate: get/counterfeiter get/stringer
	go generate ./...

build:
	go build -o bin/driftwatcher cmd/drift_watcher/main.go

test: pull/localstack
	go test ./... 

testv: pull/localstack
	go test -v ./... 

testcov: pull/localstack
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

test-specific: pull/localstack
	go test -run $(TEST_FUNCTION) $(PACKAGE_PATH)
