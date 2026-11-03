package config

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"google.golang.org/genai"
	"google.golang.org/grpc"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/client/llmclients"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/store"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/grpcserver"
)

type GeminiLLMConfig struct {
	Model            string `yaml:"model"`
	MaxRetries       int    `yaml:"max_retries"`
	RetryBaseDelayMs int    `yaml:"retry_base_delay_ms"`
}

type Config struct {
	GRPCPort     int             `yaml:"grpc_port"`
	Env          string          `yaml:"env"`
	GeminiConfig GeminiLLMConfig `yaml:"gemini"`
}

func loadConfig() (Config, error) {
	var cfg Config

	if err := configloader.Load("chatbot", &cfg); err != nil {
		return Config{}, fmt.Errorf("chatbot: load config: %w", err)
	}

	if cfg.GeminiConfig.Model == "" {
		cfg.GeminiConfig.Model = "gemini-3.5-flash-lite"
	}
	if cfg.GeminiConfig.MaxRetries < 0 {
		return Config{}, fmt.Errorf("chatbot: max_retries cannot be negative")
	}
	if cfg.GeminiConfig.RetryBaseDelayMs <= 0 {
		cfg.GeminiConfig.RetryBaseDelayMs = 250
	}

	return cfg, nil
}

func newPostgresPool() (*pgxpool.Pool, error) {
	dsn := configloader.MustGetEnv("CHATBOT_POSTGRES_DSN")

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("chatbot: connect postgres: %w", err)
	}

	return pool, nil
}

func newGeminiClient() (*genai.Client, error) {
	apiKey := configloader.MustGetEnv("GEMINI_LLM_API_KEY")

	geminiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("chatbot: initialize gemini client: %w", err)
	}

	return geminiClient, nil
}

func newLLM(geminiClient *genai.Client, cfg Config) client.LLM {
	return llmclients.NewGeminiLLM(geminiClient, cfg.GeminiConfig.Model)
}

func grpcServerConfig(cfg Config) grpcserver.Config {
	return grpcserver.Config{
		Port: cfg.GRPCPort,
	}
}

func registerHandler(server *grpc.Server, h *handler.Handler) {
	chatbotpb.RegisterChatbotServiceServer(server, h)
}

func newController(st store.Store, llm client.LLM, cfg Config) controller.Controller {
	return controller.New(st, llm, controller.RetryConfig{
		MaxRetries: cfg.GeminiConfig.MaxRetries,
		BaseDelay:  time.Duration(cfg.GeminiConfig.RetryBaseDelayMs) * time.Millisecond,
	})
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(newPostgresPool),
	fx.Provide(store.New),
	fx.Provide(newGeminiClient),
	fx.Provide(newLLM),
	fx.Provide(newController),
	fx.Provide(handler.New),
	fx.Provide(grpcServerConfig),
	grpcserver.Module,
	fx.Invoke(registerHandler),
	fx.Invoke(grpcserver.Start),
)
