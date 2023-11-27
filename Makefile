build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o split_brain_check main.go