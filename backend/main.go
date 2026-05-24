package main

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -o docs --parseDependency --parseInternal

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "ai-api-portal/backend/docs"
	"ai-api-portal/backend/internal/config"
	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/httpapi"
	"ai-api-portal/backend/internal/mailer"
	"ai-api-portal/backend/internal/proxy"
	portalstripe "ai-api-portal/backend/internal/stripe"
	"ai-api-portal/backend/internal/user"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type healthResponse struct {
	Status string `json:"status"`
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

// @title AI API Portal Backend API
// @version 1.0
// @description API documentation for AI API Portal backend service.
// @BasePath /

func main() {
	configPath := flag.String("config", "./config.yaml", "path to backend YAML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	initLogger(cfg.LogLevel)
	slog.Info("config loaded", "log_level", cfg.LogLevel, "db_driver", cfg.Database.Driver, "port", cfg.Server.Port)

	if cfg.Sub2APIBaseURL == "" {
		slog.Error("failed to load config: SUB2API_BASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := db.Open(ctx, cfg.Database.Driver, cfg.Database.EffectiveDSN())
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			slog.Error("failed to close database", "error", err)
		}
	}()
	slog.Info("database connected", "driver", cfg.Database.Driver)

	if err := db.ApplyMigrations(ctx, database, cfg.Database.Driver); err != nil {
		slog.Error("failed to apply migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")

	opts := user.ServiceOptions{
		AllowedEmailDomains:      cfg.Register.AllowedEmailDomains,
		RequireEmailVerification: cfg.Register.RequireEmailVerification,
	}
	if cfg.SMTP.Enabled {
		sender, err := mailer.NewSMTPSender(mailer.SMTPConfig{
			Host:     cfg.SMTP.Host,
			Port:     cfg.SMTP.Port,
			Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password,
			From:     cfg.SMTP.From,
			FromName: cfg.SMTP.FromName,
		})
		if err != nil {
			slog.Error("failed to init smtp sender", "error", err)
			os.Exit(1)
		}
		opts.MailSender = sender
	}
	userSvc := user.NewServiceWithOptions(database, opts)

	if cfg.Redis.Enabled {
		slog.Info("redis configured", "addr", cfg.Redis.Addr, "db", cfg.Redis.DB)
	}

	proxyClient, err := proxy.NewClientWithOptions(cfg.Sub2APIBaseURL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: cfg.Sub2APIAdminKey})
	if err != nil {
		slog.Error("failed to init sub2api proxy client", "error", err)
		os.Exit(1)
	}
	slog.Info("sub2api proxy client initialized", "base_url", cfg.Sub2APIBaseURL)

	var stripeClient *portalstripe.Client
	if cfg.Stripe.SecretKey != "" {
		stripeClient, err = portalstripe.NewClient(portalstripe.Config{
			SecretKey:      cfg.Stripe.SecretKey,
			WebhookSecret:  cfg.Stripe.WebhookSecret,
			Currency:       cfg.Stripe.Currency,
			SuccessURL:     cfg.Stripe.SuccessURL,
			CancelURL:      cfg.Stripe.CancelURL,
		})
		if err != nil {
			slog.Error("failed to init stripe client", "error", err)
			os.Exit(1)
		}
		slog.Info("stripe client initialized")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	mux.Handle("/swagger/", httpSwagger.Handler())
	httpapi.RegisterRoutesWithOptions(mux, database, httpapi.RoutesOptions{
		UserService:          userSvc,
		ProxyClient:          proxyClient,
		StripeClient:         stripeClient,
		AdminBootstrapSecret: cfg.Auth.AdminBootstrapSecret,
		SQLDialect:           cfg.Database.Driver,
	})

	handler := requestLogger(mux)

	addr := ":" + cfg.Server.Port
	slog.Info("backend server listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func initLogger(level string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slogLevel,
		AddSource: true,
	})
	slog.SetDefault(slog.New(handler))
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", sw.status,
			"duration", duration.String(),
			"remote", r.RemoteAddr,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
