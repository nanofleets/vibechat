.PHONY: build run test

build:
	docker build -t vibechat-utils .

run-local:
	go run main.go bots --count=3 --url=http://localhost:8080

test:
	go test -v ./...
