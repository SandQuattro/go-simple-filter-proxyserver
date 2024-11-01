.PHONY: build
build:
	@go build -v -o filter-proxy ./cmd/proxy/main.go