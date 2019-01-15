export GO111MODULE=on
BINARY_NAME=tftpd

all: deps build
install:
	go install cmd/$(BINARY_NAME)/$(BINARY_NAME).go
build:
	go build cmd/$(BINARY_NAME)/$(BINARY_NAME).go
test:
	go test -p 1 -v ./...
clean:
	go clean cmd/$(BINARY_NAME)/$(BINARY_NAME).go
	rm -f $(BINARY_NAME)
deps:
	go build -v ./...
upgrade:
	go get -u