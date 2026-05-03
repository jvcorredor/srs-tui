build:
    go build -o ./bin/srs ./cmd/srs

test:
    go test ./...

vet:
    go vet ./...

fmt:
    gofmt -w .

fmt-check:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted=$(gofmt -l .)
    if [ -n "$unformatted" ]; then
        echo "Unformatted files:"
        echo "$unformatted"
        exit 1
    fi

lint:
    golangci-lint run

ci: fmt-check lint vet test

clean:
    rm -rf ./bin
