package communicator

import (
	"context"
	"math"
	"time"

	"github.com/veritasvpn/lib/logging"
	"github.com/veritasvpn/services/wg-manager/internal/model"
)

type AgentClient interface {
	PushPeerUpdate(ctx context.Context, serverAddr string, action string, peerID string, publicKey string, presharedKey string, allowedIPs []string) error
}

type Communicator struct {
	client AgentClient
	log    *logging.Logger
}

func New(client AgentClient, log *logging.Logger) *Communicator {
	return &Communicator{
		client: client,
		log:    log,
	}
}

func (c *Communicator) PushPeerAdded(ctx context.Context, serverAddr string, peer *model.Peer) error {
	psk := ""
	if peer.PresharedKey != nil {
		psk = *peer.PresharedKey
	}
	return c.pushWithBackoff(ctx, serverAddr, "add", peer.ID, peer.Pubkey, psk, peer.AllowedIPs)
}

func (c *Communicator) PushPeerRemoved(ctx context.Context, serverAddr string, peerID string) error {
	return c.pushWithBackoff(ctx, serverAddr, "remove", peerID, "", "", nil)
}

func (c *Communicator) pushWithBackoff(ctx context.Context, serverAddr, action, peerID, pubkey, psk string, allowedIPs []string) error {
	maxRetries := 3
	baseDelay := 200 * time.Millisecond

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := c.client.PushPeerUpdate(callCtx, serverAddr, action, peerID, pubkey, psk, allowedIPs)
		cancel()

		if err == nil {
			c.log.Info("peer update pushed to agent",
				"server", serverAddr,
				"action", action,
				"attempt", i+1,
			)
			return nil
		}

		lastErr = err
		c.log.Warn("agent push failed, retrying",
			"attempt", i+1,
			"server", serverAddr,
			"action", action,
			"error", err.Error(),
		)

		if i < maxRetries-1 {
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(i)))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	c.log.Error("agent push failed after all retries",
		"server", serverAddr,
		"action", action,
		"error", lastErr,
	)
	return lastErr
}

type LoggingAgentClient struct {
	log *logging.Logger
}

func NewLoggingAgentClient(log *logging.Logger) AgentClient {
	return &LoggingAgentClient{log: log}
}

func (l *LoggingAgentClient) PushPeerUpdate(ctx context.Context, serverAddr string, action string, peerID string, publicKey string, presharedKey string, allowedIPs []string) error {
	l.log.Info("pushing peer update to agent (logging client)",
		"server", serverAddr,
		"action", action,
		"peer_id", peerID,
	)
	return nil
}
