package graph

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/graph/model"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/internal/helpers"
	"github.com/yaninyzwitty/gqlgen-proxy-grpc-products-service/pb"
)

// CreateCategory is the resolver for the createCategory field.
func (r *mutationResolver) CreateCategory(ctx context.Context, input model.CreateCategoryInput) (*model.Category, error) {
	createCategoryRequest := &pb.CreateCategoryRequest{
		Name:        input.Name,
		Description: input.Description,
	}

	createCategoryresponse, err := r.Conn.CreateCategory(ctx, createCategoryRequest)
	if err != nil {
		return nil, fmt.Errorf("error creating category: %v", err)
	}

	return &model.Category{
		ID:          fmt.Sprintf("%d", createCategoryresponse.Id),
		Name:        createCategoryRequest.Name,
		Description: createCategoryRequest.Description,
		CreatedAt:   createCategoryresponse.CreatedAt.AsTime().Format(time.RFC3339),
	}, nil

}

// CreateProduct is the resolver for the createProduct field.
func (r *mutationResolver) CreateProduct(ctx context.Context, input model.CreateProductInput) (*model.Product, error) {
	categoryIdInt, err := strconv.ParseUint(input.CategoryID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing category ID: %v", err)
	}

	createProductRequest := &pb.CreateProductRequest{
		Name:        input.Name,
		Description: input.Description,
		Price:       float32(input.Price),
		Stock:       input.Stock,
		CategoryId:  int64(categoryIdInt),
	}

	createdProductRes, err := r.Conn.CreateProduct(ctx, createProductRequest)
	if err != nil {
		return nil, fmt.Errorf("error creating product: %v", err)

	}

	return &model.Product{
		ID:          fmt.Sprintf("%d", createdProductRes.Product.Id),
		CategoryID:  fmt.Sprintf("%d", createdProductRes.Product.CategoryId),
		Name:        createdProductRes.Product.Name,
		Description: createProductRequest.Description,
		Price:       float64(createProductRequest.Price),
		Stock:       createProductRequest.Stock,
		CreatedAt:   createdProductRes.Product.CreatedAt.AsTime().Format(time.RFC3339),
		UpdatedAt:   createdProductRes.Product.UpdatedAt.AsTime().Format(time.RFC3339),
	}, nil

}

// GetCategory is the resolver for the getCategory field.
func (r *queryResolver) GetCategory(ctx context.Context, id string) (*model.Category, error) {
	categoryId, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing category ID: %v", err)
	}

	getCategoryRequest := &pb.GetCategoryRequest{
		Id: int64(categoryId),
	}
	category, err := r.Conn.GetCategory(ctx, getCategoryRequest)
	if err != nil {
		return nil, fmt.Errorf("error getting category: %v", err)
	}

	return &model.Category{
		ID:          fmt.Sprintf("%d", category.Id),
		Name:        category.Name,
		Description: category.Description,
		CreatedAt:   category.CreatedAt.AsTime().Format(time.RFC3339),
	}, nil
}

// GetProduct is the resolver for the getProduct field.
func (r *queryResolver) GetProduct(ctx context.Context, categoryID string, productID string) (*model.Product, error) {
	categoryIdInt, err := strconv.ParseUint(categoryID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing category ID: %v", err)
	}
	productIdInt, err := strconv.ParseUint(productID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing product ID: %v", err)
	}

	productRes, err := r.Conn.GetProduct(ctx, &pb.GetProductRequest{
		CategoryId: int64(categoryIdInt),
		ProductId:  int64(productIdInt),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting product: %v", err)
	}

	return &model.Product{
		ID:          fmt.Sprintf("%d", productRes.Product.Id),
		CategoryID:  fmt.Sprintf("%d", productRes.Product.CategoryId),
		Name:        productRes.Product.Name,
		Description: productRes.Product.Description,
		Price:       float64(productRes.Product.Price),
		Stock:       productRes.Product.Stock,
		CreatedAt:   productRes.Product.CreatedAt.AsTime().Format(time.RFC3339),
		UpdatedAt:   productRes.Product.UpdatedAt.AsTime().Format(time.RFC3339),
	}, nil
}

// ListProducts is the resolver for the listProducts field.
func (r *queryResolver) ListProducts(ctx context.Context, categoryID string, pageSize *int32, pagingState *string) (*model.ListProductsResponse, error) {
	if categoryID == "" {
		return nil, fmt.Errorf("categoryID is required")
	}

	// Convert categoryID to bigint (int64)
	categoryId, err := strconv.ParseInt(categoryID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing category ID: %v", err)
	}

	// Set default page size if not provided
	limit := int32(10)
	if pageSize != nil && *pageSize > 0 {
		limit = *pageSize
	}

	// Decode paging state if provided
	var decodedPagingState []byte
	if pagingState != nil {
		decodedPagingState = helpers.DecodePagingState(pagingState)
	}

	resp, err := r.Conn.ListProducts(ctx, &pb.ListProductsRequest{
		CategoryId:  categoryId, // Using bigint (int64)
		PagingState: decodedPagingState,
		PageSize:    limit,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch products: %v", err)
	}

	// Convert response to GraphQL model
	products := make([]*model.Product, len(resp.Products))
	for i, p := range resp.Products {
		products[i] = &model.Product{
			ID:          strconv.FormatInt(p.Id, 10),
			CategoryID:  strconv.FormatInt(p.CategoryId, 10),
			Name:        p.Name,
			Description: p.Description,
			Price:       float64(p.Price),
			Stock:       p.Stock,
			CreatedAt:   p.CreatedAt.AsTime().Format(time.RFC3339),
			UpdatedAt:   p.UpdatedAt.AsTime().Format(time.RFC3339),
		}
	}

	return &model.ListProductsResponse{
		Products:    products,
		PagingState: helpers.EncodePagingState(resp.PagingState),
	}, nil
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
