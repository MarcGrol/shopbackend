

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

run:
	go install && shopbackend && open http://localhost:8082/

.PHONY: all clean install test gen fmt run
