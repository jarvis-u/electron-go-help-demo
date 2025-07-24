Version			  ?= 0.0.1

build:
	GOARCH=amd64 GOOS=darwin go build -ldflags "-s -w -X main.version=${Version}" -o ktctl-helper main.go
