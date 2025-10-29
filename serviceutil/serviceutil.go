// Package serviceutil provides service helper functions
package serviceutil

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/science-computing/service-common-golang/apputil"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const metricsPublishPort string = "8080"
const restPublishPort string = "8081"
const grpcPublishPort string = "8090"

var log = apputil.InitLogging()

// ErrInvalidArgument indicates, that one or more provided arguments are invalid, e.g. required data is missing
var ErrInvalidArgument = errors.New("One ore more request arguments are invalid")

// Service defines values to start a GRPC (and a REST) service
type Service struct {
	Name               string
	GrpcPublishPort    string
	GrpcOptions        []grpc.ServerOption
	RestPort           string
	MetricsPort        string
	WaitGroup          sync.WaitGroup
	Client             interface{}
	NewClientFunc      func(*grpc.ClientConn) interface{}
	RegisterServerFunc func(s *grpc.Server, srv interface{})
	RegisterClientFunc func(ctx context.Context, mux *runtime.ServeMux, client interface{}) error
	Service            interface{}
	ServeHTTP          bool // enables REST endpoints
	SwaggerJsonPath    string
}

// Start runs service with GRPC and REST service endpoints.
// REST is served if Service.ServeHttp is true (currently fixed on port 8080)
func (service *Service) Start() {
	if service.MetricsPort == "" {
		service.MetricsPort = metricsPublishPort
	}
	if service.RestPort == "" {
		service.RestPort = restPublishPort
	}
	if service.GrpcPublishPort == "" {
		service.GrpcPublishPort = grpcPublishPort
	}

	//TODO check service config

	// start http metrics server
	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{},
		))
		http.ListenAndServe(":"+service.MetricsPort, nil)
	}()

	// start grpc server
	service.WaitGroup.Add(1)
	go func() {
		if err := service.startGRPC(); err != nil {
			log.Fatal(err.Error())
		}
		service.WaitGroup.Done()
	}()

	// start http server with grpc <-> rest gateway
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
	address := "localhost:" + service.GrpcPublishPort
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial grpc service at %q: %v", address, err)
	}
	defer conn.Close()

	// register grpc-gateway
	rmux := runtime.NewServeMux()
	client := service.NewClientFunc(conn)
	err = service.RegisterClientFunc(ctx, rmux, client)
	if err != nil {
		return fmt.Errorf("failed to start HTTP service [%w]", err)
	}

	// serve swagger file
	mux := http.NewServeMux()
	mux.Handle("/", rmux)

	if service.SwaggerJsonPath != "" {
		log.Infof("Using %s as swagger.json", service.SwaggerJsonPath)
		mux.HandleFunc("/swagger.json", func(writer http.ResponseWriter, request *http.Request) {
			http.ServeFile(writer, request, service.SwaggerJsonPath)
		})
	} else {
		log.Infof("No swagger.json specifed")
	}

	// serve swagger-ui
	swaggerMux := http.NewServeMux()
	swaggerMux.Handle("/", rmux)
	fs := http.FileServer(http.Dir("web"))
	mux.Handle("/swagger-ui/", http.StripPrefix("/swagger-ui", fs))

	log.Infof("HTTP server start listening on port %v", service.RestPort)
	return http.ListenAndServe("0.0.0.0:"+service.RestPort, mux)
}

func (service *Service) startGRPC() error {
	// start listening for grpc
	listen, err := net.Listen("tcp", ":"+service.GrpcPublishPort)
	if err != nil {
		return fmt.Errorf("failed to create Listen for GRPC service [%w]", err)
	}

	// create new grpc server
	server := grpc.NewServer(service.GrpcOptions...)

	reflection.Register(server)
	grpc.EnableTracing = true

	// register service
	service.RegisterServerFunc(server, service.Service)

	log.Infof("GRPC server start listening on port %v", service.GrpcPublishPort)
	return server.Serve(listen)
}

// GetServiceConnection establishes connection to GRPC service at given URL.
// We are not waiting, til the service is up (no grpc.WithBlock())
func GetServiceConnection(serviceAddress string) (service *grpc.ClientConn, err error) {
	return GetServiceConnectionWithDialOptions(serviceAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// GetServiceConnectionWithDialOptions establishes connection to GRPC service at given URL with given dial options.
func GetServiceConnectionWithDialOptions(serviceAddress string, dialOptions ...grpc.DialOption) (service *grpc.ClientConn, err error) {
	service, err = grpc.NewClient(serviceAddress, dialOptions...)

	if err != nil {
		log.Errorf("failed to connect to %q: %v", serviceAddress, err)
		return nil, err
	}

	log.Debugf("Connection established to service at %q", service.Target())
	return service, nil
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
		grpcErr = status.Errorf(codes.NotFound, "instance not found")
	case err == ErrInvalidArgument:
		grpcErr = status.Errorf(codes.InvalidArgument, message)
	default:
		grpcErr = status.Errorf(codes.Internal, "an internal error occurred")
	}

	log.Errorf("an error occurred: %s\npublic error: %v\ninternal error: %v", message, grpcErr, err)
	return grpcErr
}

// CloseContexts is deprecated
func CloseContexts() {
}
