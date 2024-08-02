VERSION=$(shell git describe --tags --always)
PKG := github.com/morsuning/system-usability-detection/version
LDFLAGS = -s -w
LDFLAGS += -X $(PKG).Version=$(VERSION)
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o system-usability-detection main.go

build_arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o system-usability-detection-arm main.go
