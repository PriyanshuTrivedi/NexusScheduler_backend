package config

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/middleware"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/router"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	resourcepb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
)

type Config struct {
	HTTPPort          int        `yaml:"http_port"`
	Env               string     `yaml:"env"`
	IdentityGRPCAddr  string     `yaml:"identity_grpc_addr"`
	ResourceGRPCAddr  string     `yaml:"resource_grpc_addr"`
	BookingGRPCAddr   string     `yaml:"booking_grpc_addr"`
	RedisAddr         string     `yaml:"redis_addr"`
	JWT               JWTConfig  `yaml:"jwt"`
	RateLimit         int        `yaml:"rate_limit"`
	RateWindowSeconds int        `yaml:"rate_window_seconds"`
	CORS              CORSConfig `yaml:"cors"`
}
type CORSConfig struct {
	AllowedOrigin string `yaml:"allowed_origin"`
}
type JWTConfig struct {
	Issuer   string `yaml:"issuer"`
	TTLHours int    `yaml:"ttl_hours"`
}

func loadConfig() (Config, error) {
	var c Config
	if err := configloader.Load("api-gateway", &c); err != nil {
		return c, fmt.Errorf("api-gateway: load config: %w", err)
	}
	if c.HTTPPort == 0 {
		c.HTTPPort = 8080
	}
	if c.RateLimit == 0 {
		c.RateLimit = 60
	}
	if c.RateWindowSeconds == 0 {
		c.RateWindowSeconds = 60
	}
	return c, nil
}
func jwtSecret() string {
	return configloader.MustGetEnv("API_GATEWAY_JWT_SECRET")
}

type LocationIQKey string

func locationIQAPIKey() LocationIQKey {
	return LocationIQKey(configloader.MustGetEnv("LOCATIONIQ_API_KEY"))
}
func newVerifier(c Config, secret string) *middleware.Verifier {
	return middleware.NewVerifier(secret, c.JWT.Issuer)
}
func newIssuer(c Config, secret string) *middleware.TokenIssuer {
	ttl := time.Duration(c.JWT.TTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return middleware.NewTokenIssuer(secret, c.JWT.Issuer, ttl)
}
func newRedis(c Config) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: c.RedisAddr})
}
func newLimiter(r *redis.Client) middleware.RateLimiter {
	return middleware.NewRedisRateLimiter(r)
}

type grpcClients struct {
	identity client.IdentityClient
	resource client.ResourceClient
	booking  client.BookingClient
	conns    []*grpc.ClientConn
}

func newClients(lc fx.Lifecycle, c Config) (grpcClients, error) {
	iConn, err := grpc.NewClient(c.IdentityGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return grpcClients{}, err
	}
	rConn, err := grpc.NewClient(c.ResourceGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = iConn.Close()
		return grpcClients{}, err
	}
	bConn, err := grpc.NewClient(c.BookingGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		_ = iConn.Close()
		_ = rConn.Close()
		return grpcClients{}, err
	}
	lc.Append(fx.Hook{OnStop: func(ctx context.Context) error {
		var first error
		for _, conn := range []*grpc.ClientConn{iConn, rConn, bConn} {
			if err := conn.Close(); err != nil && first == nil {
				first = err
			}
		}
		return first
	}})
	return grpcClients{identity: identitypb.NewIdentityServiceClient(iConn), resource: resourcepb.NewResourceServiceClient(rConn), booking: bookingpb.NewBookingServiceClient(bConn), conns: []*grpc.ClientConn{iConn, rConn, bConn}}, nil
}

func newHandler(c grpcClients, issuer *middleware.TokenIssuer, apiKey LocationIQKey) *handler.Handler {
	return handler.New(c.identity, c.resource, c.booking, issuer, string(apiKey))
}
func newHTTPHandler(h *handler.Handler, v *middleware.Verifier, l middleware.RateLimiter, c Config) http.Handler {
	routerHandler := router.New(h, v, l, router.Config{
		RateLimit:  c.RateLimit,
		RateWindow: time.Duration(c.RateWindowSeconds) * time.Second,
	},
	)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{c.CORS.AllowedOrigin},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
		},
		AllowCredentials: true,
	})

	return corsHandler.Handler(routerHandler)
}
func start(lc fx.Lifecycle, c Config, h http.Handler) {
	srv := &http.Server{Addr: fmt.Sprintf(":%d", c.HTTPPort), Handler: h, ReadHeaderTimeout: 5 * time.Second}
	lc.Append(fx.Hook{OnStart: func(ctx context.Context) error { go srv.ListenAndServe(); return nil }, OnStop: func(ctx context.Context) error { return srv.Shutdown(ctx) }})
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(jwtSecret),
	fx.Provide(locationIQAPIKey),
	fx.Provide(newVerifier),
	fx.Provide(newIssuer),
	fx.Provide(newRedis),
	fx.Provide(newLimiter),
	fx.Provide(newClients),
	fx.Provide(newHandler),
	fx.Provide(newHTTPHandler),
	fx.Invoke(start),
)
