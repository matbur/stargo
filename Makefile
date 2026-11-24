.PHONY: test vet race bench build run run-hello-world run-chat-room run-auth test-examples

test:
	go test ./...

vet:
	go vet ./...

race:
	go test -race ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...

build:
	go build ./...

run:
	go run ./examples/hello

run-hello-world:
	go run ./examples/hello-world

run-chat-room:
	cd examples/chat-room && go run .

run-auth:
	go run ./examples/auth

test-examples:
	go test ./examples/hello-world
	go test ./examples/auth
	cd examples/chat-room && go vet ./... && go test -race ./... && go build ./...
