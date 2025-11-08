package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chirik/products/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	totalRequests   int64
	failedRequests  int64
	successRequests int64
)

func main() {
	var (
		serverAddr = flag.String("addr", "localhost:50051", "gRPC server address")
		vusers     = flag.Int("vusers", 10, "Number of virtual users")
		rpm        = flag.Int("rpm", 60, "Requests per minute")
		duration   = flag.Duration("duration", 5*time.Minute, "Test duration")
	)
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting load test",
		zap.String("server", *serverAddr),
		zap.Int("vusers", *vusers),
		zap.Int("rpm", *rpm),
		zap.Duration("duration", *duration),
	)

	// Calculate request interval per user
	requestsPerSecond := float64(*rpm) / 60.0
	requestInterval := time.Duration(float64(time.Second) / (requestsPerSecond / float64(*vusers)))

	logger.Info("Load test configuration",
		zap.Float64("rps", requestsPerSecond),
		zap.Duration("interval", requestInterval),
	)

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	var wg sync.WaitGroup

	// Start metrics reporter
	go reportMetrics(ctx, logger, *duration)

	// Start virtual users
	for i := 0; i < *vusers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			runVirtualUser(ctx, *serverAddr, userID, requestInterval, logger)
		}(i)
	}

	// Wait for all virtual users to complete
	wg.Wait()

	logger.Info("Load test completed",
		zap.Int64("total_requests", atomic.LoadInt64(&totalRequests)),
		zap.Int64("success_requests", atomic.LoadInt64(&successRequests)),
		zap.Int64("failed_requests", atomic.LoadInt64(&failedRequests)),
	)
}

func runVirtualUser(ctx context.Context, serverAddr string, userID int, interval time.Duration, logger *zap.Logger) {
	// Create gRPC connection
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect", zap.Int("user", userID), zap.Error(err))
		return
	}
	defer conn.Close()

	client := proto.NewProductsServiceClient(conn)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			makeRequest(ctx, client, userID, logger)
		}
	}
}

func makeRequest(ctx context.Context, client proto.ProductsServiceClient, userID int, logger *zap.Logger) {
	atomic.AddInt64(&totalRequests, 1)

	// Randomly choose between different operations
	operation := rand.Intn(100)
	var err error

	switch {
	case operation < 70: // 70% list products
		req := &proto.ListProductsRequest{
			Page:     int32(rand.Intn(5) + 1),
			PageSize: int32(rand.Intn(20) + 10),
		}
		if rand.Float32() < 0.3 {
			categories := []string{"Electronics", "Furniture", "Appliances", "Sports"}
			req.Category = categories[rand.Intn(len(categories))]
		}
		if rand.Float32() < 0.2 {
			searchTerms := []string{"laptop", "chair", "coffee", "shoes", "mouse"}
			req.SearchQuery = searchTerms[rand.Intn(len(searchTerms))]
		}
		_, err = client.ListProducts(ctx, req)

	case operation < 90: // 20% get product
		productIDs := []string{"1", "2", "3", "4", "5"}
		req := &proto.GetProductRequest{
			Id: productIDs[rand.Intn(len(productIDs))],
		}
		_, err = client.GetProduct(ctx, req)

	default: // 10% create product
		req := &proto.CreateProductRequest{
			Name:        fmt.Sprintf("Test Product %d", time.Now().UnixNano()),
			Description: "Load test product",
			Price:       rand.Float64()*1000 + 10,
			Category:    "Test",
			Stock:       int32(rand.Intn(100)),
		}
		_, err = client.CreateProduct(ctx, req)
	}

	if err != nil {
		atomic.AddInt64(&failedRequests, 1)
		logger.Debug("Request failed", zap.Int("user", userID), zap.Error(err))
	} else {
		atomic.AddInt64(&successRequests, 1)
	}
}

func reportMetrics(ctx context.Context, logger *zap.Logger, duration time.Duration) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	lastTotal := int64(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			total := atomic.LoadInt64(&totalRequests)
			success := atomic.LoadInt64(&successRequests)
			failed := atomic.LoadInt64(&failedRequests)

			elapsed := time.Since(startTime)
			requestsSinceLastReport := total - lastTotal
			rps := float64(requestsSinceLastReport) / 10.0

			logger.Info("Metrics",
				zap.Duration("elapsed", elapsed),
				zap.Int64("total", total),
				zap.Int64("success", success),
				zap.Int64("failed", failed),
				zap.Float64("rps", rps),
				zap.Float64("success_rate", float64(success)/float64(total)*100),
			)

			lastTotal = total
		}
	}
}

