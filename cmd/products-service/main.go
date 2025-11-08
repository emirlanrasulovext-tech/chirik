package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/chirik/products/internal/config"
	"github.com/chirik/products/internal/observability"
	"github.com/chirik/products/internal/repository"
	"github.com/chirik/products/internal/server"
	"github.com/chirik/products/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger, err := observability.NewLogger(cfg.LogFilePath)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting products service",
		zap.String("port", cfg.GRPCPort),
		zap.String("redis_addr", cfg.RedisAddr),
	)

	// Initialize observability
	shutdown, err := observability.Init(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize observability", zap.Error(err))
	}
	defer shutdown()

	// Initialize repository
	repo, err := repository.NewRedisRepository(cfg.RedisAddr, logger)
	if err != nil {
		logger.Fatal("Failed to create repository", zap.Error(err))
	}
	defer repo.Close()

	// Initialize gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(observability.UnaryServerInterceptor(logger)),
	)

	// Register service
	productsServer := server.NewProductsServer(repo, logger)
	proto.RegisterProductsServiceServer(grpcServer, productsServer)
	reflection.Register(grpcServer)

	// Start server
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen", zap.Error(err))
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve", zap.Error(err))
		}
	}()

	logger.Info("Products service is running",
		zap.String("address", lis.Addr().String()),
	)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down products service...")
	grpcServer.GracefulStop()
	logger.Info("Products service stopped")
}
