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
	-I$(GOPATH)/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
	--go_out=plugins=grpc:gen/api \
    --grpc-gateway_out=logtostderr=true:gen/api \
	--swagger_out=logtostderr=true:gen/api \
	api/exampleservice.proto