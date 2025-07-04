COUNTERFEITER      := $(shell command -v counterfeiter 2> /dev/null)
STRINGER 					:= $(shell command -v stringer 2> /dev/null)

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

