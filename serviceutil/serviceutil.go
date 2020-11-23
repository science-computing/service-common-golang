// Package serviceutil provides service helper functions
package serviceutil

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/apex/log"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const httpPublishPort string = "8080"
const grpcPublishPort string = "8090"

// ErrInvalidArgument indicates, that one or more provided arguments are invalid, e.g. required data is missing
var ErrInvalidArgument = errors.New("One ore more request arguments are invalid")

var logger *log.Entry

// Service defines values to start a GRPC (and a REST) service
type Service struct {
	Name               string
	GrpcPublishPort    string
	httpPort           string
	WaitGroup          sync.WaitGroup
	Client             interface{}
	NewClientFunc      func(*grpc.ClientConn) interface{}
	RegisterServerFunc func(s *grpc.Server, srv interface{})
	RegisterClientFunc func(ctx context.Context, mux *runtime.ServeMux, client interface{}) error
	Service            interface{}
	ServeHTTP          bool // enables REST endpoints
}

// Start runs service with GRPC and REST service endpoints.
// REST is served if Service.ServeHttp is true (currently fixed on port 8080)
func (service *Service) Start() {
	service.httpPort = httpPublishPort

	//TODO check service config

	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{},
		))
		http.ListenAndServe(":8080", nil)
	}()

	// start grpc
	service.WaitGroup.Add(1)
	go func() {
		if err := service.startGRPC(); err != nil {
			log.Fatal(err.Error())
		}
		service.WaitGroup.Done()
	}()

	// start rest
	if service.ServeHTTP {
		service.WaitGroup.Add(1)
		go func() {
			if err := service.startREST(); err != nil {
				log.Fatal(err.Error())
			}
			service.WaitGroup.Done()
		}()
	}
	log.Infof("Service [%v] started", service.Name)
}

func (service *Service) startREST() error {
	// create top level context
	ctx := context.Background()

	// create context that's closed when cancel() ist called
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// connect to GRPC server
	conn, err := grpc.Dial("localhost:"+service.GrpcPublishPort, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("Failed to dial GRPC service [%w]", err)
	}
	defer conn.Close()

	// register grpc-gateway
	rmux := runtime.NewServeMux()
	client := service.NewClientFunc(conn)
	err = service.RegisterClientFunc(ctx, rmux, client)
	if err != nil {
		return fmt.Errorf("Failed to start HTTP service [%w]", err)
	}

	// serve swagger file
	mux := http.NewServeMux()
	mux.Handle("/", rmux)

	// TODO: swagger web files are not relative to serviceutil anymore and thus not found
	// TODO: swagger files must be copied when building container
	mux.HandleFunc("/swagger.json", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, "api/dataset.swagger.json")
	})

	// serve swagger-ui
	swaggerMux := http.NewServeMux()
	swaggerMux.Handle("/", rmux)
	fs := http.FileServer(http.Dir("web"))
	mux.Handle("/swagger-ui/", http.StripPrefix("/swagger-ui", fs))

	log.Infof("HTTP server start listening on port %v", service.httpPort)
	return http.ListenAndServe("localhost:"+service.httpPort, mux)
}

func (service *Service) startGRPC() error {
	// start listening for grpc
	listen, err := net.Listen("tcp", ":"+service.GrpcPublishPort)

	if err != nil {
		return fmt.Errorf("Failed to create Listen for GRPC service [%w]", err)
	}

	// create new grpc server
	server := grpc.NewServer()
	grpc.EnableTracing = true

	// register service
	service.RegisterServerFunc(server, service.Service)

	log.Infof("GRPC server start listening on port %v", service.GrpcPublishPort)
	return server.Serve(listen)
}

// GetServiceConnection establishes connection to GRPC service at given URL.
// We are not waiting, til the service is up (no grpc.WithBlock())
func GetServiceConnection(serviceAddress string) (service *grpc.ClientConn, err error) {
	service, err = grpc.Dial(serviceAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to [%v]", err)
	}

	log.Infof("Connection established to service at [%v]", service.Target())
	return service, err
}

// AsGrpcError returns a GRPC error, mapping internal errors and returning
// codes.Internal as default error.
// The error is logged
// If the given err is nil AsGrpcError returns nil
func AsGrpcError(err error, message string, messageArgs ...interface{}) error {
	if err == nil {
		return nil
	}

	// format message
	message = fmt.Sprintf(message, messageArgs)

	// TODO add error mappings
	var grpcErr error
	switch {
	case err == sql.ErrNoRows:
		grpcErr = status.Errorf(codes.NotFound, "Instance not found")
	case err == ErrInvalidArgument:
		grpcErr = status.Errorf(codes.InvalidArgument, message)
	default:
		grpcErr = status.Errorf(codes.Internal, "An internal error occurred")
	}

	log.Errorf("MESSAGE: [%v] PUBLIC ERROR: [%v] INTERNAL ERROR: [%v]", message, grpcErr.Error(), err)
	return grpcErr
}

// CloseContexts is deprecated
func CloseContexts() {
}
