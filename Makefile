.PHONY: dev build linux clean deps

deps:
	go mod tidy

dev: deps
	go run .

build: deps
	go build -o dartcounter.exe .

linux: deps
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o dartcounter .

clean:
	rm -f dartcounter dartcounter.exe
