package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/veritasvpn/lib/config"
	"github.com/veritasvpn/lib/logging"
	"github.com/veritasvpn/services/wg-manager/internal/communicator"
	"github.com/veritasvpn/services/wg-manager/internal/handler"
	"github.com/veritasvpn/services/wg-manager/internal/repository"
	"github.com/veritasvpn/services/wg-manager/internal/scheduler"
	"github.com/veritasvpn/services/wg-manager/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()

	log, err := logging.New(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to postgres", "error", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("postgres ping failed", "error", err)
	}
	log.Info("connected to postgres")

	redisRepo, err := repository.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to create redis client", "error", err)
	}

	if err := redisRepo.Client().Ping(ctx).Err(); err != nil {
		log.Fatal("redis ping failed", "error", err)
	}
	log.Info("connected to redis")

	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatal("failed to connect to nats", "error", err)
	}
	defer nc.Close()
	log.Info("connected to nats", "url", cfg.NatsURL)

	pgRepo := repository.NewPostgres(pool)
	sched := scheduler.New(pgRepo, log)
	agentClient := communicator.NewLoggingAgentClient(log)
	comm := communicator.New(agentClient, log)

	svc := service.New(pgRepo, redisRepo, sched, comm, nc, cfg.AgentAuthToken, log)

	wgHandler := handler.NewWireGuardHandler(svc, log)
	agentHandler := handler.NewAgentHandler(svc, log)

	grpcAddr := cfg.GRPCServerAddr()
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("failed to listen", "addr", grpcAddr, "error", err)
	}

	grpcSrv := grpc.NewServer()
	reflection.Register(grpcSrv)

	handler.RegisterWireGuardServiceServer(grpcSrv, wgHandler)
	handler.RegisterAgentServiceServer(grpcSrv, agentHandler)

	errCh := make(chan error, 1)
	go func() {
		log.Info("starting gRPC server", "addr", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		log.Error("server error", "error", err)
	}

	cancel()
	grpcSrv.GracefulStop()
	lis.Close()
	log.Info("server stopped")
}
