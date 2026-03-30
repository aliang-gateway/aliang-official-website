package main

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -o docs --parseDependency --parseInternal

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
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
		log.Fatalf("failed to load config: %v", err)
	}
	if cfg.Sub2APIBaseURL == "" {
		log.Fatalf("failed to load config: SUB2API_BASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := db.Open(ctx, cfg.Database.Driver, cfg.Database.EffectiveDSN())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.ApplyMigrations(ctx, database, cfg.Database.Driver); err != nil {
		log.Fatalf("failed to apply migrations: %v", err)
	}

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
			log.Fatalf("failed to init smtp sender: %v", err)
		}
		opts.MailSender = sender
	}
	userSvc := user.NewServiceWithOptions(database, opts)

	if cfg.Redis.Enabled {
		log.Printf("redis configured at %s (db=%d)", cfg.Redis.Addr, cfg.Redis.DB)
	}

	proxyClient, err := proxy.NewClientWithOptions(cfg.Sub2APIBaseURL, &http.Client{Timeout: proxy.RequestTimeout}, proxy.ClientOptions{AdminAPIKey: cfg.Sub2APIAdminKey})
	if err != nil {
		log.Fatalf("failed to init sub2api proxy client: %v", err)
	}

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
			log.Fatalf("failed to init stripe client: %v", err)
		}
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

	addr := ":" + cfg.Server.Port
	log.Printf("backend server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
