package controller

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gocql/gocql"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/snowflake"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProductController struct {
	pb.UnimplementedProductServiceServer
	session *gocql.Session
}

func NewProductController(session *gocql.Session) *ProductController {
	return &ProductController{session: session}
}

func (c *ProductController) CreateCategory(ctx context.Context, req *pb.CreateCategoryRequest) (*pb.CreateCategoryResponse, error) {
	if req.Name == "" || req.Description == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name and description are required")
	}

	categoryId, err := snowflake.GenerateID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate category id")
	}

	now := time.Now()
	createCategoryQuery := `INSERT INTO products_keyspace_v2.categories(id, name, description, created_at) VALUES(?, ?, ?, ?)`

	if err := c.session.Query(createCategoryQuery, categoryId, req.Name, req.Description, now).WithContext(ctx).Exec(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create category: %v", err)
	}

	return &pb.CreateCategoryResponse{
		Id:          int64(categoryId),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   timestamppb.New(now),
	}, nil
}

func (c *ProductController) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	if req.Name == "" || req.Description == "" || req.Price == 0 || req.CategoryId == 0 || req.Stock == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "name, description, price, stock, and category id are required")
	}

	productId, err := snowflake.GenerateID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate product id")
	}

	now := time.Now()
	outboxID := gocql.TimeUUID()
	bucket := now.Format("2006-01-02")
	eventType := "create_product_event"

	product := &pb.Product{
		Id:          int64(productId),
		CategoryId:  req.CategoryId,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		CreatedAt:   timestamppb.New(now),
		UpdatedAt:   timestamppb.New(now),
	}

	payload, err := json.Marshal(product)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal product: %v", err)
	}

	batch := c.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	batch.Query(
		`INSERT INTO products_keyspace_v2.products 
		(id, name, description, price, stock, category_id, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		productId, req.Name, req.Description, req.Price, req.Stock, req.CategoryId, now, now,
	)

	batch.Query(
		`INSERT INTO products_keyspace_v2.products_outbox 
		(id, bucket, payload, event_type) 
		VALUES (?, ?, ?, ?)`,
		outboxID, bucket, payload, eventType,
	)

	if err := c.session.ExecuteBatch(batch); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create product: %v", err)
	}

	return &pb.CreateProductResponse{Product: product}, nil
}

func (c *ProductController) GetCategory(ctx context.Context, req *pb.GetCategoryRequest) (*pb.GetCategoryResponse, error) {
	if req.Id == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	getCategoryQuery := `SELECT id, name, description, created_at FROM products_keyspace_v2.categories WHERE id = ?`
	var categoryId int64
	var name, description string
	var createdAt time.Time

	if err := c.session.Query(getCategoryQuery, req.Id).WithContext(ctx).Scan(&categoryId, &name, &description, &createdAt); err != nil {
		if err == gocql.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "category not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get category: %v", err)
	}

	return &pb.GetCategoryResponse{
		Id:          categoryId,
		Name:        name,
		Description: description,
		CreatedAt:   timestamppb.New(createdAt),
	}, nil
}

func (c *ProductController) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	if req.CategoryId == 0 || req.ProductId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "category id and product id are required")
	}

	var product pb.Product
	var createdAt, updatedAt time.Time

	getProductQuery := `SELECT id, name, description, price, stock, category_id, created_at, updated_at FROM products_keyspace_v2.products WHERE category_id = ? AND id = ?`
	err := c.session.Query(getProductQuery, req.CategoryId, req.ProductId).WithContext(ctx).Scan(
		&product.Id, &product.Name, &product.Description, &product.Price, &product.Stock, &product.CategoryId, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == gocql.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "product not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to fetch product: %v", err)
	}

	product.CreatedAt = timestamppb.New(createdAt)
	product.UpdatedAt = timestamppb.New(updatedAt)

	return &pb.GetProductResponse{Product: &product}, nil
}

func (c *ProductController) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	if req.CategoryId == 0 || req.PageSize <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "category id and page size are required")
	}

	getProductsQuery := `SELECT id, name, description, price, stock, created_at, updated_at FROM products_keyspace_v2.products WHERE category_id = ?`

	query := c.session.Query(getProductsQuery, req.CategoryId).WithContext(ctx).PageSize(int(req.PageSize))
	if req.PagingState != nil {
		query = query.PageState(req.PagingState)
	}

	iter := query.Iter()
	var products []*pb.Product
	var id int64
	var name, description string
	var price float32
	var stock int32
	var createdAt, updatedAt time.Time

	for iter.Scan(&id, &name, &description, &price, &stock, &createdAt, &updatedAt) {
		products = append(products, &pb.Product{
			Id:          id,
			Name:        name,
			Description: description,
			Price:       price,
			Stock:       stock,
			CreatedAt:   timestamppb.New(createdAt),
			UpdatedAt:   timestamppb.New(updatedAt),
		})
	}

	if err := iter.Close(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list products: %v", err)
	}

	return &pb.ListProductsResponse{
		Products:    products,
		PagingState: iter.PageState(),
	}, nil
}
