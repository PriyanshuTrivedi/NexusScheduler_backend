package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/fx"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/config"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/internal/tools"
)

func newMCPHTTPHandler(apiGateway client.APIGatewayClient) http.Handler {
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "nexus-scheduler",
			Version: "1.0.0",
		},
		nil,
	)

	tools.RegisterResourceTools(mcpServer, apiGateway)
	tools.RegisterBookingTools(mcpServer, apiGateway)

	streamableHandler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return mcpServer
		},
		&mcp.StreamableHTTPOptions{
			Stateless: true,
		},
	)

	mux := http.NewServeMux()
	handler := identityMiddleware(streamableHandler)

	mux.Handle("/mcp", handler)
	mux.Handle("/mcp/", handler)

	return mux
}

func identityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
		role := strings.ToLower(strings.TrimSpace(r.Header.Get("X-User-Role")))

		if userID == "" || (role != "client" && role != "resource") {
			http.Error(w, "valid X-User-ID and X-User-Role headers are required", http.StatusUnauthorized)
			return
		}

		ctx := entity.WithIdentity(r.Context(), entity.Identity{
			UserID: userID,
			Role:   role,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func startHTTPServer(
	lc fx.Lifecycle,
	cfg config.Config,
	handler http.Handler,
) {
	addr := fmt.Sprintf("%s:%d", cfg.ListenHost, cfg.HTTPPort)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var listener net.Listener

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("mcp: listen on %s: %w", addr, err)
			}

			listener = ln

			go func() {
				err := server.Serve(listener)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Printf("mcp: HTTP server failed: %v", err)
				}
			}()

			log.Printf("mcp: listening on %s", addr)
			return nil
		},

		OnStop: func(ctx context.Context) error {
			if listener == nil {
				return nil
			}

			return server.Shutdown(ctx)
		},
	})
}
