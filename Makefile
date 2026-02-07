.PHONY: all clean fmt lint generate test

all: clean fmt lint test
publish: lint clean

clean:
	go clean -i .
	rm coverage.* || true
	echo ''>go.sum
	go mod tidy

fmt:
	gofmt -w .
	goimports -w .

lint: clean
	go generate ./...
	#go vet ./...
	#golint ./...
	golangci-lint run -c golangci.yml -v --print-resources-usage
	gocyclo -top 5 .
	gocyclo -top 5 internal/bytecode
	gocyclo -top 5 internal/patch
	gocyclo -top 5 internal/proxy
	gocyclo -top 5 internal/iface
	gocyclo -top 5 internal/hack
	gocyclo -top 5 internal/bytecode/memory

generate:
	go generate ./...

test: clean generate
	go test -gcflags=all=-l -coverpkg=./... -coverprofile=coverage.data ./... -run=^TestUnit.*$
	go tool cover -html=coverage.data -o coverage.html
