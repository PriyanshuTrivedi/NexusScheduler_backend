package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/client/cache"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/client/dbbyte"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/store"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/grpcserver"
)

type DbbyteConfig struct {
	Bucket  string `yaml:"bucket"`
	BaseUrl string `yaml:"base_url"`
}

type Config struct {
	GRPCPort     int          `yaml:"grpc_port"`
	Env          string       `yaml:"env"`
	DbbyteConfig DbbyteConfig `yaml:"dbbyte"`
}

func loadConfig() (Config, error) {
	var cfg Config

	if err := configloader.Load("identity", &cfg); err != nil {
		return Config{}, fmt.Errorf("identity: load config: %w", err)
	}

	if cfg.DbbyteConfig.Bucket == "" {
		return Config{}, fmt.Errorf("identity: dbbyte bucket is required")
	}

	return cfg, nil
}

func grpcServerConfig(cfg Config) grpcserver.Config {
	return grpcserver.Config{Port: cfg.GRPCPort}
}

func newPostgresPool() (*pgxpool.Pool, error) {
	dsn := configloader.MustGetEnv("IDENTITY_POSTGRES_DSN")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("identity: connect postgres: %w", err)
	}
	return pool, nil
}

func newRedisClient() *redis.Client {
	addr := configloader.MustGetEnv("REDIS_ADDR")
	return redis.NewClient(&redis.Options{Addr: addr})
}

func newDBByteClient(cfg Config) (dbbyte.Client, error) {
	apiKey := configloader.MustGetEnv("DBBYTE_API_KEY")
	client, err := dbbyte.New(apiKey, cfg.DbbyteConfig.BaseUrl, cfg.DbbyteConfig.Bucket)
	if err != nil {
		return nil, fmt.Errorf("identity: create dbbyte client: %w", err)
	}
	return client, nil
}

func registerHandler(server *grpc.Server, h *handler.Handler) {
	identitypb.RegisterIdentityServiceServer(server, h)
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(grpcServerConfig),
	fx.Provide(newPostgresPool),
	fx.Provide(newRedisClient),
	fx.Provide(store.New),
	fx.Provide(cache.New),
	fx.Provide(newDBByteClient),
	fx.Provide(controller.New),
	fx.Provide(controller.NewObjectStorageController),
	fx.Provide(handler.New),
	grpcserver.Module,
	fx.Invoke(registerHandler),
	fx.Invoke(grpcserver.Start),
)
