TARGET = bin/example-service

GOPATH   := $(shell go env GOPATH)

all: clean bin

clean:
	rm -rf bin; \
	rm -rf gen; \
	mkdir gen; \
	mkdir gen/api; \

bin: proto
	go build -o $(TARGET) main.go

proto:
	protoc \
	--proto_path=./api \
	-I$(GOPATH)/src/github.com/googleapis/googleapis \
	--go_out=gen/api \
    --go-grpc_out=gen/api \
    --grpc-gateway_out=logtostderr=true:gen/api \
	--openapiv2_out=logtostderr=true:gen/api \
	api/exampleservice.proto
