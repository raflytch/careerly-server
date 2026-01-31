.PHONY: dev build run clean tidy

dev:
	air

build:
	go build -o ./tmp/main.exe ./cmd/main.go

run:
	go run ./cmd/main.go

clean:
	rm -rf ./tmp

tidy:
	go mod tidy

install-air:
	go install github.com/air-verse/air@latest
