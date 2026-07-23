package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/veritasvpn/lib/logging"
	"github.com/veritasvpn/services/wg-manager/internal/communicator"
	"github.com/veritasvpn/services/wg-manager/internal/model"
	"github.com/veritasvpn/services/wg-manager/internal/repository"
	"github.com/veritasvpn/services/wg-manager/internal/scheduler"
)

type PeerConfig struct {
	PeerID          string
	ServerID        string
	ServerHostname  string
	ServerPublicKey string
	ServerEndpoint  string
	AssignedIP      string
	DNSServer       string
	AllowedIPs      []string
}

type Service struct {
	postgres     *repository.Postgres
	redis        *repository.Redis
	scheduler    *scheduler.Scheduler
	communicator *communicator.Communicator
	natsConn     *nats.Conn
	authToken    string
	log          *logging.Logger
}

func New(
	postgres *repository.Postgres,
	redis *repository.Redis,
	scheduler *scheduler.Scheduler,
	communicator *communicator.Communicator,
	natsConn *nats.Conn,
	authToken string,
	log *logging.Logger,
) *Service {
	return &Service{
		postgres:     postgres,
		redis:        redis,
		scheduler:    scheduler,
		communicator: communicator,
		natsConn:     natsConn,
		authToken:    authToken,
		log:          log,
	}
}

func (s *Service) RegisterServer(ctx context.Context, hostname, publicKey, publicIP string, wgPort int32, region, city, country, authToken string) (*model.Server, error) {
	if authToken != s.authToken {
		return nil, fmt.Errorf("invalid agent auth token")
	}

	subnet, err := s.allocateSubnet(ctx)
	if err != nil {
		return nil, err
	}

	dnsServer := strings.Replace(strings.Replace(subnet, ".0/24", ".1", 1), ".0.0/24", ".0.1", 1)

	srv := &model.Server{
		Hostname:  hostname,
		Region:    region,
		City:      city,
		Country:   country,
		PublicIP:  publicIP,
		WGPort:    wgPort,
		PublicKey: publicKey,
		Status:    "online",
		Capacity:  100,
		WGSubnet:  subnet,
		DNSServer: dnsServer,
	}

	if err := s.postgres.RegisterServer(ctx, srv); err != nil {
		return nil, fmt.Errorf("register server: %w", err)
	}

	s.log.Info("server registered",
		"server_id", srv.ID,
		"hostname", hostname,
		"subnet", subnet,
	)

	s.publishEvent("server.registered", map[string]interface{}{
		"server_id": srv.ID,
		"hostname":  srv.Hostname,
		"region":    srv.Region,
		"city":      srv.City,
		"country":   srv.Country,
		"subnet":    subnet,
	})

	return srv, nil
}

func (s *Service) HandleHeartbeat(ctx context.Context, serverID string, peerCount int32, loadFactor float64, rxBytes, txBytes int64) error {
	if err := s.postgres.UpdateServerLoad(ctx, serverID, peerCount, loadFactor); err != nil {
		return fmt.Errorf("update server load: %w", err)
	}

	if err := s.postgres.UpdateServerStatus(ctx, serverID, "online"); err != nil {
		s.log.Warn("failed to set server status to online", "server_id", serverID, "error", err)
	}

	s.log.Debug("heartbeat processed",
		"server_id", serverID,
		"load_factor", loadFactor,
		"peer_count", peerCount,
	)

	s.publishEvent("server.heartbeat", map[string]interface{}{
		"server_id":   serverID,
		"peer_count":  peerCount,
		"load_factor": loadFactor,
		"rx_bytes":    rxBytes,
		"tx_bytes":    txBytes,
	})

	return nil
}

func (s *Service) CreatePeer(ctx context.Context, accountID, publicKey, preferredRegion string) (*PeerConfig, error) {
	srv, err := s.scheduler.SelectServer(ctx, preferredRegion)
	if err != nil {
		return nil, err
	}

	assignedIP, err := s.redis.AllocateIP(ctx, srv.ID, srv.WGSubnet)
	if err != nil {
		return nil, fmt.Errorf("allocate ip: %w", err)
	}

	peer := &model.Peer{
		AccountID:  accountID,
		ServerID:   srv.ID,
		Pubkey:     publicKey,
		AllowedIPs: []string{assignedIP},
		AssignedIP: assignedIP,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	if err := s.postgres.CreatePeer(ctx, peer); err != nil {
		_ = s.redis.ReleaseIP(ctx, srv.ID, assignedIP)
		return nil, fmt.Errorf("create peer: %w", err)
	}

	endpoint := fmt.Sprintf("%s:%d", srv.PublicIP, srv.WGPort)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.communicator.PushPeerAdded(bgCtx, endpoint, peer); err != nil {
			s.log.Warn("agent notification failed for new peer",
				"peer_id", peer.ID,
				"server", endpoint,
				"error", err.Error(),
			)
		}
	}()

	s.log.Info("peer created",
		"peer_id", peer.ID,
		"account_id", accountID,
		"server_id", srv.ID,
		"assigned_ip", assignedIP,
	)

	s.publishEvent("peer.created", map[string]interface{}{
		"peer_id":      peer.ID,
		"account_id":   accountID,
		"server_id":    srv.ID,
		"assigned_ip":  assignedIP,
		"server_endpoint": endpoint,
	})

	return &PeerConfig{
		PeerID:          peer.ID,
		ServerID:        srv.ID,
		ServerHostname:  srv.Hostname,
		ServerPublicKey: srv.PublicKey,
		ServerEndpoint:  endpoint,
		AssignedIP:      assignedIP,
		DNSServer:       srv.DNSServer,
		AllowedIPs:      []string{assignedIP},
	}, nil
}

func (s *Service) DeletePeer(ctx context.Context, peerID, accountID string) error {
	peer, err := s.postgres.GetPeer(ctx, peerID, accountID)
	if err != nil {
		return fmt.Errorf("get peer for deletion: %w", err)
	}

	if err := s.postgres.DeletePeer(ctx, peerID, accountID); err != nil {
		return fmt.Errorf("delete peer: %w", err)
	}

	srv, err := s.postgres.GetServer(ctx, peer.ServerID)
	if err == nil {
		_ = s.redis.ReleaseIP(ctx, peer.ServerID, peer.AssignedIP)

		endpoint := fmt.Sprintf("%s:%d", srv.PublicIP, srv.WGPort)
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.communicator.PushPeerRemoved(bgCtx, endpoint, peer.ID); err != nil {
				s.log.Warn("agent notification failed for deleted peer",
					"peer_id", peerID,
					"server", endpoint,
					"error", err.Error(),
				)
			}
		}()
	} else {
		s.log.Warn("could not get server for peer deletion", "server_id", peer.ServerID, "error", err)
	}

	s.log.Info("peer deleted", "peer_id", peerID, "account_id", accountID)

	s.publishEvent("peer.deleted", map[string]interface{}{
		"peer_id":    peerID,
		"account_id": accountID,
		"server_id":  peer.ServerID,
	})

	return nil
}

func (s *Service) GetPeer(ctx context.Context, peerID, accountID string) (*model.Peer, *model.Server, error) {
	peer, err := s.postgres.GetPeer(ctx, peerID, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("get peer: %w", err)
	}

	srv, err := s.postgres.GetServer(ctx, peer.ServerID)
	if err != nil {
		s.log.Warn("could not get server for peer", "server_id", peer.ServerID, "error", err)
		return peer, nil, nil
	}

	return peer, srv, nil
}

func (s *Service) ListPeers(ctx context.Context, accountID string) ([]model.Peer, error) {
	peers, err := s.postgres.ListPeersByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	return peers, nil
}

func (s *Service) ListServers(ctx context.Context) ([]model.Server, error) {
	servers, err := s.postgres.ListOnlineServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	return servers, nil
}

func (s *Service) allocateSubnet(ctx context.Context) (string, error) {
	counter, err := s.redis.Client().Incr(ctx, "wg:subnet_counter").Result()
	if err != nil {
		return "", fmt.Errorf("allocate subnet: %w", err)
	}
	return fmt.Sprintf("10.%d.0.0/24", counter), nil
}

func (s *Service) publishEvent(subject string, data map[string]interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		s.log.Warn("failed to marshal event", "subject", subject, "error", err)
		return
	}
	if err := s.natsConn.Publish(subject, payload); err != nil {
		s.log.Warn("failed to publish event", "subject", subject, "error", err)
	}
}
