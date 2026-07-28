.PHONY: build test fmt vet ci


build:
	go build -o bin/forage ./cmd/forage

test:
	go test ./...

fmt:
	@bad=$$(gofmt -l .); \
	if [ -n "$$bad" ]; then \
		echo "The following files are not formatted correctly:"; \
		echo "$$bad"; \
		exit 1; \
	fi

vet:
	go vet ./...

ci: fmt vet test
