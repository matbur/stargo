.PHONY: test vet race build run run-hello-world run-chat-room test-examples

test:
	go test ./...

vet:
	go vet ./...

race:
	go test -race ./...

build:
	go build ./...

run:
	go run ./examples/hello

run-hello-world:
	go run ./examples/hello-world

run-chat-room:
	cd examples/chat-room && go run .

test-examples:
	go test ./examples/hello-world
	cd examples/chat-room && go vet ./... && go test -race ./... && go build ./...
