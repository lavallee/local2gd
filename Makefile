.PHONY: build test clean

VERSION ?= dev

build:
	go build -ldflags "-s -w -X github.com/lavallee/local2gd/cmd.version=$(VERSION)" -o local2gd .

test:
	go test ./...

clean:
	rm -f local2gd

vet:
	go vet ./...

release-snapshot:
	goreleaser release --snapshot --clean
