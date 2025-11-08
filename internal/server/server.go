package server

import (
	"context"

	"github.com/chirik/products/internal/repository"
	"github.com/chirik/products/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProductsServer struct {
	proto.UnimplementedProductsServiceServer
	repo   repository.Repository
	logger *zap.Logger
}

func NewProductsServer(repo repository.Repository, logger *zap.Logger) *ProductsServer {
	return &ProductsServer{
		repo:   repo,
		logger: logger,
	}
}

func (s *ProductsServer) ListProducts(ctx context.Context, req *proto.ListProductsRequest) (*proto.ListProductsResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	products, total, err := s.repo.ListProducts(
		ctx,
		req.Page,
		req.PageSize,
		req.Category,
		req.SearchQuery,
	)
	if err != nil {
		s.logger.Error("Failed to list products", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to list products: %v", err)
	}

	protoProducts := make([]*proto.Product, len(products))
	for i, p := range products {
		protoProducts[i] = &proto.Product{
			Id:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			Category:    p.Category,
			Stock:       p.Stock,
			CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return &proto.ListProductsResponse{
		Products: protoProducts,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *ProductsServer) GetProduct(ctx context.Context, req *proto.GetProductRequest) (*proto.Product, error) {
	if req.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "product id is required")
	}

	product, err := s.repo.GetProduct(ctx, req.Id)
	if err != nil {
		s.logger.Error("Failed to get product", zap.String("id", req.Id), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "product not found: %v", err)
	}

	return &proto.Product{
		Id:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Category:    product.Category,
		Stock:       product.Stock,
		CreatedAt:   product.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (s *ProductsServer) CreateProduct(ctx context.Context, req *proto.CreateProductRequest) (*proto.Product, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "product name is required")
	}
	if req.Price < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "product price must be non-negative")
	}

	product := &repository.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		Stock:       req.Stock,
	}

	if err := s.repo.CreateProduct(ctx, product); err != nil {
		s.logger.Error("Failed to create product", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to create product: %v", err)
	}

	return &proto.Product{
		Id:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Category:    product.Category,
		Stock:       product.Stock,
		CreatedAt:   product.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

