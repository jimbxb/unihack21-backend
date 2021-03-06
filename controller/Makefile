build: 
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o autossl

run:
	go run cmd/main.go

clean:
	rm -rf bin/

all: build
