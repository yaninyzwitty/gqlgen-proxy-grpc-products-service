package graph

import "github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Conn pb.ProductServiceClient
}
