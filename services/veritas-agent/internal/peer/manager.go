package peer

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/veritasvpn/services/veritas-agent/internal/wireguard"
)

type PeerConfig struct {
	PeerID       string
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string
}

type Manager struct {
	wg    *wireguard.Manager
	mu    sync.RWMutex
	peers map[string]*PeerConfig
}

func New(wgManager *wireguard.Manager) *Manager {
	return &Manager{
		wg:    wgManager,
		peers: make(map[string]*PeerConfig),
	}
}

func (m *Manager) AddPeer(peerID, pubkey, psk string, allowedIPs []string) error {
	ipNets := cidrsToIPNets(allowedIPs)

	var pskPtr *string
	if psk != "" {
		pskPtr = &psk
	}

	if err := m.wg.AddPeer(pubkey, ipNets, pskPtr); err != nil {
		return fmt.Errorf("peer add %s: %w", peerID, err)
	}

	m.mu.Lock()
	m.peers[pubkey] = &PeerConfig{
		PeerID:       peerID,
		PublicKey:    pubkey,
		PresharedKey: psk,
		AllowedIPs:   allowedIPs,
	}
	m.mu.Unlock()

	return nil
}

func (m *Manager) RemovePeer(pubkey string) error {
	if err := m.wg.RemovePeer(pubkey); err != nil {
		return fmt.Errorf("peer remove %s: %w", pubkey, err)
	}

	m.mu.Lock()
	delete(m.peers, pubkey)
	m.mu.Unlock()
	return nil
}

func (m *Manager) SyncPeers(desired []PeerConfig) error {
	want := make(map[string]*PeerConfig, len(desired))
	for i := range desired {
		want[desired[i].PublicKey] = &desired[i]
	}

	m.mu.Lock()
	for pubkey := range m.peers {
		if _, ok := want[pubkey]; !ok {
			m.mu.Unlock()
			if err := m.wg.RemovePeer(pubkey); err != nil {
				return fmt.Errorf("sync remove %s: %w", pubkey, err)
			}
			m.mu.Lock()
			delete(m.peers, pubkey)
		}
	}

	for pubkey, cfg := range want {
		if _, ok := m.peers[pubkey]; !ok {
			ipNets := cidrsToIPNets(cfg.AllowedIPs)

			var pskPtr *string
			if cfg.PresharedKey != "" {
				pskPtr = &cfg.PresharedKey
			}

			m.mu.Unlock()
			if err := m.wg.AddPeer(pubkey, ipNets, pskPtr); err != nil {
				return fmt.Errorf("sync add %s: %w", pubkey, err)
			}
			m.mu.Lock()
			m.peers[pubkey] = cfg
		}
	}
	m.mu.Unlock()

	return nil
}

func (m *Manager) GetStats() (rxBytes, txBytes int64, peerCount int32) {
	m.mu.RLock()
	count := int32(len(m.peers))
	m.mu.RUnlock()

	wgPeers, err := m.wg.ListPeers()
	if err != nil {
		return 0, 0, count
	}

	for _, p := range wgPeers {
		rxBytes += p.RXBytes
		txBytes += p.TXBytes
	}

	return rxBytes, txBytes, count
}

func cidrsToIPNets(cidrs []string) []net.IPNet {
	ipNets := make([]net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		c := cidr
		if !strings.Contains(c, "/") {
			c = c + "/32"
		}
		_, ipNet, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		ipNets = append(ipNets, *ipNet)
	}
	return ipNets
}
