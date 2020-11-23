package main

import (
	"context"
	exampleapi "example/gen/api"

	"example/internal/exampleapiimpl"

	"github.com/science-computing/service-common-golang/apputil"
	"github.com/science-computing/service-common-golang/serviceutil"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const projectName = "example"
const serviceName = "exampleservice"

const myConfigKey = "myConfig"
const grpcPublishPortConfigKey = "grpcPublishPort"

var log = apputil.InitLogging()

func main() {
	// parse command line flags. Do this even if you dont use flags here,
	// to initialize flags used by other packages, e.g. serviceutil
	pflag.Parse()

	//init config with mandatory parameters
	apputil.InitConfig(projectName, serviceName, []string{myConfigKey})

	var server exampleapi.ExampleServiceServer
	myConfig := viper.GetString(myConfigKey)

	log.Infof("myConfig is %s.", myConfig)

	server = &exampleapiimpl.ExampleService{ExampleField: myConfig}

	// configure service
	service := &serviceutil.Service{}
	service.Name = serviceName
	service.GrpcPublishPort = viper.GetString(grpcPublishPortConfigKey)
	service.NewClientFunc = func(conn *grpc.ClientConn) interface{} { return exampleapi.NewExampleServiceClient(conn) }
	service.RegisterClientFunc = func(ctx context.Context, mux *runtime.ServeMux, client interface{}) error {
		return exampleapi.RegisterExampleServiceHandlerClient(ctx, mux, client.(exampleapi.ExampleServiceClient))
	}
	service.RegisterServerFunc = func(s *grpc.Server, _ interface{}) {
		exampleapi.RegisterExampleServiceServer(s, server.(exampleapi.ExampleServiceServer))
	}

	service.Start()

	//close connections to db etc.
	defer serviceutil.CloseContexts()

	//wait forever
	service.WaitGroup.Wait()
}
