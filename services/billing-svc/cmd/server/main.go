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
	"github.com/veritasvpn/services/billing-svc/internal/firebaseauth"
	"github.com/veritasvpn/services/billing-svc/internal/handler"
	"github.com/veritasvpn/services/billing-svc/internal/migrate"
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

	if err := migrate.Up(ctx, dbPool); err != nil {
		log.Fatal("failed to apply billing migrations", zap.Error(err))
	}
	log.Info("billing migrations applied")

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

	var (
		invoiceCreator provider.InvoiceCreator
		btcpay         *provider.BTCPayProvider
		mock           *provider.MockBTCPayProvider
	)

	if cfg.UseMockBTCPay() {
		mock = provider.NewMockBTCPayProvider(cfg.BillingPublicURL)
		invoiceCreator = mock
		log.Warn("BTCPay mock mode enabled — no real Bitcoin invoices")
	} else {
		btcpay = provider.NewBTCPayProvider(
			log,
			cfg.BTCPayServerURL,
			cfg.BTCPayAPIKey,
			cfg.BTCPayStoreID,
			cfg.BTCPayWebhookSecret,
			cfg.CheckoutSuccessURL,
		)
		invoiceCreator = btcpay
		log.Info("BTCPay provider configured", zap.String("url", cfg.BTCPayServerURL), zap.String("store", cfg.BTCPayStoreID))
	}

	svc := service.New(log, db, natsConn, invoiceCreator, btcpay, mock, service.BillingConfig{
		PremiumPriceUSDCents: cfg.PremiumPriceUSDCents,
		PremiumPeriodDays:    cfg.PremiumPeriodDays,
	})

	firebase := firebaseauth.NewVerifier(cfg.FirebaseProjectID)
	billingHandler := handler.NewBillingHandler(log, svc, firebase, cfg.AllowedCORSOrigins(), cfg.CheckoutSuccessURL)

	mux := http.NewServeMux()
	billingHandler.RegisterRoutes(mux)

	// Background expiry worker
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			n, err := svc.ExpireDueSubscriptions(context.Background())
			if err != nil {
				log.Error("expiry worker failed", zap.Error(err))
			} else if n > 0 {
				log.Info("expired premium subscriptions", zap.Int("count", n))
			}
			<-ticker.C
		}
	}()

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
