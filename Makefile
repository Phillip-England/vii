test:
	go test ./...

dev:
	go run main.go

docs:
	bun ./www/**/*.html