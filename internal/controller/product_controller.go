package controller

import (
	"context"

	"github.com/gocql/gocql"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"
)

type ProductController struct {
	pb.UnimplementedProductServiceServer
	session *gocql.Session
}

func NewProductController(session *gocql.Session) *ProductController {
	return &ProductController{session: session}
}

func (c *ProductController) CreateCategory(ctx context.Context, req *pb.CreateCategoryRequest) (*pb.CreateCategoryResponse, error) {
	return nil, nil
}

func (c *ProductController) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	return nil, nil
}

func (c *ProductController) GetCategory(ctx context.Context, req *pb.GetCategoryRequest) (*pb.GetCategoryResponse, error) {
	return nil, nil
}

func (c *ProductController) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	return nil, nil
}

func (c *ProductController) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	return nil, nil
}
