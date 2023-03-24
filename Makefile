

all: gen fmt test install

clean:
	go clean ./...

install:
	go install ./...

test:
	go test ./...

gen:
	go generate ./...

fmt:
	find . -name '*.go' -exec goimports -l -w {} \;

.PHONY: all clean install test gen fmt
