package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/veritasvpn/lib/config"
	"github.com/veritasvpn/lib/logging"
	"github.com/veritasvpn/services/billing-svc/internal/handler"
	"github.com/veritasvpn/services/billing-svc/internal/provider"
	"github.com/veritasvpn/services/billing-svc/internal/repository"
	"github.com/veritasvpn/services/billing-svc/internal/service"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	log, err := logging.New(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatal("database ping failed", zap.Error(err))
	}
	log.Info("connected to PostgreSQL")

	var natsConn *nats.Conn
	if cfg.NatsURL != "" {
		nc, err := nats.Connect(cfg.NatsURL)
		if err != nil {
			log.Warn("failed to connect to NATS, continuing without events", zap.Error(err))
		} else {
			natsConn = nc
			log.Info("connected to NATS")
			defer natsConn.Close()
		}
	}

	db := repository.NewPostgres(dbPool)
	stripeProvider := provider.NewStripeProvider(log, cfg.StripeSecretKey, cfg.StripeWebhookSecret)
	btcpayProvider := provider.NewBTCPayProvider(log, cfg.BTCPayServerURL, cfg.BTCPayAPIKey)
	svc := service.New(log, db, natsConn, stripeProvider, btcpayProvider)
	billingHandler := handler.NewBillingHandler(log, svc)

	mux := http.NewServeMux()
	billingHandler.RegisterRoutes(mux)

	addr := cfg.ServerAddr()

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("billing-svc starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down billing-svc...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("server forced to shutdown", zap.Error(err))
	}

	log.Info("billing-svc stopped")
}
