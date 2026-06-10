BINARY := linko
MODULE := boot.dev/linko
GIT_SHA := $(shell git rev-parse HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -X $(MODULE)/internal/build.GitSHA=$(GIT_SHA) -X $(MODULE)/internal/build.BuildTime=$(BUILD_TIME)

.PHONY: build run test clean

build:
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY)

run: build
	LINKO_LOG_FILE=linko.access.log ENV=development ./$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
