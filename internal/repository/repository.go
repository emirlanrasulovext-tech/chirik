package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RediSearch/redisearch-go/v2/redisearch"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Category    string    `json:"category"`
	Stock       int32     `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

type Repository interface {
	CreateProduct(ctx context.Context, product *Product) error
	GetProduct(ctx context.Context, id string) (*Product, error)
	ListProducts(ctx context.Context, page, pageSize int32, category, searchQuery string) ([]*Product, int32, error)
	Close() error
}

type RedisRepository struct {
	client        *redis.Client
	search        *redisearch.Client
	logger        *zap.Logger
	indexName     string
	searchEnabled bool
}

const (
	productsKeyPrefix = "product:"
	defaultIndexName  = "products-index"
	minSeedProducts   = 5
)

var seedProducts = []*Product{
	{
		ID:          "seed-1",
		Name:        "Laptop Pro 15",
		Description: "High-performance laptop with 16GB RAM and 512GB SSD",
		Price:       1299.99,
		Category:    "Electronics",
		Stock:       50,
	},
	{
		ID:          "seed-2",
		Name:        "Wireless Mouse",
		Description: "Ergonomic wireless mouse with long battery life",
		Price:       29.99,
		Category:    "Electronics",
		Stock:       200,
	},
	{
		ID:          "seed-3",
		Name:        "Office Chair",
		Description: "Comfortable ergonomic office chair with lumbar support",
		Price:       199.99,
		Category:    "Furniture",
		Stock:       30,
	},
	{
		ID:          "seed-4",
		Name:        "Coffee Maker",
		Description: "Automatic drip coffee maker with programmable timer",
		Price:       79.99,
		Category:    "Appliances",
		Stock:       75,
	},
	{
		ID:          "seed-5",
		Name:        "Running Shoes",
		Description: "Lightweight running shoes with breathable mesh",
		Price:       89.99,
		Category:    "Sports",
		Stock:       120,
	},
}

func NewRedisRepository(addr string, logger *zap.Logger) (*RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	repo := &RedisRepository{
		client:    client,
		logger:    logger,
		indexName: defaultIndexName,
	}

	if err := repo.detectRediSearch(ctx); err != nil {
		logger.Warn("RediSearch module not available; search features disabled", zap.Error(err))
	} else {
		repo.searchEnabled = true
		repo.search = redisearch.NewClient(addr, repo.indexName)
	}

	// Create search index if it doesn't exist
	if err := repo.createIndex(ctx); err != nil {
		logger.Warn("Failed to create search index, continuing anyway", zap.Error(err))
	}

	// Seed initial data if needed
	if err := repo.seedData(ctx); err != nil {
		logger.Warn("Failed to seed data", zap.Error(err))
	}

	if err := repo.verifySeedData(ctx); err != nil {
		logger.Warn("Product data verification failed", zap.Error(err))
	}

	return repo, nil
}

func (r *RedisRepository) createIndex(ctx context.Context) error {
	if !r.searchEnabled || r.search == nil {
		return nil
	}

	schema := redisearch.NewSchema(redisearch.DefaultOptions).
		AddField(redisearch.NewTextField("name")).
		AddField(redisearch.NewTextField("description")).
		AddField(redisearch.NewTextField("category")).
		AddField(redisearch.NewNumericField("price")).
		AddField(redisearch.NewNumericField("stock"))

	if err := r.search.CreateIndex(schema); err != nil {
		// Index might already exist, which is fine
		r.logger.Debug("Index creation returned error (might already exist)", zap.Error(err))
	}
	return nil
}

func (r *RedisRepository) seedData(ctx context.Context) error {
	keys, err := r.client.Keys(ctx, productsKeyPrefix+"*").Result()
	if err != nil {
		return fmt.Errorf("failed to list product keys: %w", err)
	}

	existing := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		id := strings.TrimPrefix(key, productsKeyPrefix)
		existing[id] = struct{}{}
	}

	for _, product := range seedProducts {
		if _, ok := existing[product.ID]; ok {
			continue
		}
		seed := *product
		if seed.CreatedAt.IsZero() {
			seed.CreatedAt = time.Now()
		}
		if err := r.CreateProduct(ctx, &seed); err != nil {
			return fmt.Errorf("failed to seed product %s: %w", product.ID, err)
		}
		existing[seed.ID] = struct{}{}
	}

	count := len(existing)
	for count < minSeedProducts {
		id := fmt.Sprintf("seed-auto-%d", count+1)
		if _, ok := existing[id]; ok {
			count++
			continue
		}
		product := &Product{
			ID:          id,
			Name:        fmt.Sprintf("Sample Product %d", count+1),
			Description: "Automatically generated sample product",
			Price:       49.99 + float64(count),
			Category:    "Sample",
			Stock:       20 + int32(count),
			CreatedAt:   time.Now(),
		}
		if err := r.CreateProduct(ctx, product); err != nil {
			return fmt.Errorf("failed to auto-seed product %s: %w", product.ID, err)
		}
		existing[id] = struct{}{}
		count++
	}

	r.logger.Info("Ensured product seed data present", zap.Int("count", count))
	return nil
}

func (r *RedisRepository) CreateProduct(ctx context.Context, product *Product) error {
	if product.ID == "" {
		product.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if product.CreatedAt.IsZero() {
		product.CreatedAt = time.Now()
	}

	key := r.keyFor(product.ID)
	data, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("failed to marshal product: %w", err)
	}

	if err := r.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to set product: %w", err)
	}

	// Index in RedisSearch
	if r.searchEnabled && r.search != nil {
		doc := redisearch.NewDocument(key, 1.0)
		doc.Set("name", product.Name).
			Set("description", product.Description).
			Set("category", product.Category).
			Set("price", product.Price).
			Set("stock", product.Stock)

		if err := r.search.Index([]redisearch.Document{doc}...); err != nil {
			r.logger.Warn("Failed to index product", zap.Error(err))
		}
	}

	return nil
}

func (r *RedisRepository) GetProduct(ctx context.Context, id string) (*Product, error) {
	key := r.keyFor(id)
	data, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("product not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	var product Product
	if err := json.Unmarshal([]byte(data), &product); err != nil {
		return nil, fmt.Errorf("failed to unmarshal product: %w", err)
	}

	return &product, nil
}

func (r *RedisRepository) ListProducts(ctx context.Context, page, pageSize int32, category, searchQuery string) ([]*Product, int32, error) {
	useSearch := searchQuery != "" && r.searchEnabled && r.search != nil

	if useSearch {
		query := redisearch.NewQuery(searchQuery)
		if category != "" {
			query = redisearch.NewQuery(fmt.Sprintf("%s @category:{%s}", searchQuery, category))
		}
		query.SetSortBy("price", false)
		query.Limit(int((page-1)*pageSize), int(pageSize))

		docs, totalResults, err := r.search.Search(query)
		if err != nil {
			return nil, 0, fmt.Errorf("search failed: %w", err)
		}

		products := make([]*Product, 0, len(docs))
		for _, doc := range docs {
			data, err := r.client.Get(ctx, doc.Id).Result()
			if err != nil {
				r.logger.Warn("Failed to get product", zap.String("key", doc.Id), zap.Error(err))
				continue
			}

			var product Product
			if err := json.Unmarshal([]byte(data), &product); err != nil {
				r.logger.Warn("Failed to unmarshal product", zap.String("key", doc.Id), zap.Error(err))
				continue
			}

			products = append(products, &product)
		}

		return products, int32(totalResults), nil
	}

	allKeys, err := r.client.Keys(ctx, productsKeyPrefix+"*").Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get keys: %w", err)
	}

	searchQueryLower := strings.ToLower(searchQuery)
	filtered := make([]*Product, 0, len(allKeys))

	for _, key := range allKeys {
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			r.logger.Warn("Failed to get product", zap.String("key", key), zap.Error(err))
			continue
		}

		var product Product
		if err := json.Unmarshal([]byte(data), &product); err != nil {
			r.logger.Warn("Failed to unmarshal product", zap.String("key", key), zap.Error(err))
			continue
		}

		if category != "" && product.Category != category {
			continue
		}

		if searchQuery != "" {
			nameMatch := strings.Contains(strings.ToLower(product.Name), searchQueryLower)
			descMatch := strings.Contains(strings.ToLower(product.Description), searchQueryLower)
			if !nameMatch && !descMatch {
				continue
			}
		}

		filtered = append(filtered, &product)
	}

	total := int32(len(filtered))
	if total == 0 {
		return []*Product{}, 0, nil
	}

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	start := int((page - 1) * pageSize)
	if start >= len(filtered) {
		return []*Product{}, total, nil
	}

	end := start + int(pageSize)
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total, nil
}

func (r *RedisRepository) Close() error {
	return r.client.Close()
}

func (r *RedisRepository) keyFor(id string) string {
	return fmt.Sprintf("%s%s", productsKeyPrefix, id)
}

func (r *RedisRepository) detectRediSearch(ctx context.Context) error {
	if _, err := r.client.Do(ctx, "FT._LIST").Result(); err != nil {
		return err
	}
	return nil
}

func (r *RedisRepository) verifySeedData(ctx context.Context) error {
	products, total, err := r.ListProducts(ctx, 1, int32(minSeedProducts), "", "")
	if err != nil {
		return fmt.Errorf("failed to verify products listing: %w", err)
	}

	if total < int32(minSeedProducts) {
		return fmt.Errorf("insufficient seed data: have %d products, expected at least %d", total, minSeedProducts)
	}

	if len(products) == 0 {
		return fmt.Errorf("products list empty after seeding")
	}

	r.logger.Info("Verified product catalog", zap.Int32("count", total))
	return nil
}
