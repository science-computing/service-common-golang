// Package elasticapiimpl provides ElasticSearch backed implementation of metadataservice api functions
package exampleapiimpl

import (
	"context"

	"github.com/science-computing/service-common-golang/apputil"
	"github.com/science-computing/service-common-golang/serviceutil"

	exampleapi "example/gen/api"

	"google.golang.org/protobuf/types/known/emptypb"
)

var log = apputil.InitLogging()

// ExampleService just contains a field for demonstation purposes
type ExampleService struct {
	exampleapi.UnimplementedExampleServiceServer
	ExampleField string
}

// Print is just a demo
func (service *ExampleService) Print(ctx context.Context, inp *exampleapi.PrintInput) (*emptypb.Empty, error) {
	log.Infof("Printing [%v]", inp.Text)
	var err error
	return &emptypb.Empty{}, serviceutil.AsGrpcError(err, "Failed to print [%v]", inp.Text)
}
