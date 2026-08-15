// Package grpcserver provides the *grpc.Server bootstrap shared by every
// service: health checking, reflection, and an fx.Lifecycle hook that opens
// the listener on OnStart and calls GracefulStop on OnStop. Each service's
// module.go provides Config (just the port) and registers its own proto
// server on the *grpc.Server via fx.Invoke before calling Start.
package grpcserver

import (
	"context"
	"fmt"
	"log"
	"net"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Config is the minimal, service-agnostic shape this package needs. Each
// service keeps its own richer Config struct (grpc_port, env, log_level,
// ...) in its own module.go; that file maps GRPCPort into this type so
// grpcserver never has to import any individual service's config.
type Config struct {
	Port int
}

func newServer() *grpc.Server {
	s := grpc.NewServer()
	reflection.Register(s)
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(s, healthSrv)
	return s
}

// Module provides the *grpc.Server. Each service's module.go includes this
// alongside its own providers, then registers its proto server via
// fx.Invoke(registerHandler) before fx.Invoke(Start).
var Module = fx.Options(fx.Provide(newServer))

// Start is registered via fx.Invoke in each service's module.go, after that
// service's proto server has been attached to *grpc.Server. It listens on
// Config.Port for the lifetime of the fx app and stops gracefully on
// shutdown — fx calls OnStart/OnStop itself, nothing else needs to invoke
// this directly.
func Start(lc fx.Lifecycle, server *grpc.Server, cfg Config) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
			if err != nil {
				return fmt.Errorf("grpcserver: listen on port %d: %w", cfg.Port, err)
			}
			go func() {
				if err := server.Serve(lis); err != nil {
					log.Printf("grpcserver: Serve exited: %v", err)
				}
			}()
			log.Printf("grpcserver: listening on :%d", cfg.Port)
			return nil
		},
		OnStop: func(context.Context) error {
			server.GracefulStop()
			return nil
		},
	})
}
