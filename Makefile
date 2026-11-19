.PHONY: build test check race install clean

build:
	go build -trimpath -o bin/wraith ./cmd/wraith

test:
	go test ./...
	go test -count=1 -timeout=2m github.com/yuin/gopher-lua

race:
	go test -race -shuffle=on -count=1 ./...
	go test -count=1 -timeout=2m github.com/yuin/gopher-lua

check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test -race -shuffle=on -count=1 ./...
	go test -count=1 -timeout=2m github.com/yuin/gopher-lua

install: build
	install -Dm755 bin/wraith "$(HOME)/.local/bin/wraith"

clean:
	rm -rf bin dist coverage.out
