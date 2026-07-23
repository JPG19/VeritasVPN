package wireguard

import (
	"fmt"
	"net"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type PeerInfo struct {
	PublicKey  string
	AllowedIPs []net.IPNet
	RXBytes    int64
	TXBytes    int64
}

type Manager struct {
	client *wgctrl.Client
	iface  string
}

func NewManager(ifaceName string) (*Manager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("wgctrl: failed to create client: %w", err)
	}
	return &Manager{client: client, iface: ifaceName}, nil
}

func (m *Manager) AddPeer(pubkey string, allowedIPs []net.IPNet, psk *string) error {
	key, err := wgtypes.ParseKey(pubkey)
	if err != nil {
		return fmt.Errorf("wgctrl: invalid public key %q: %w", pubkey, err)
	}

	peerCfg := wgtypes.PeerConfig{
		PublicKey:  key,
		AllowedIPs: allowedIPs,
		ReplaceAllowedIPs: true,
	}

	if psk != nil && *psk != "" {
		pskKey, err := wgtypes.ParseKey(*psk)
		if err != nil {
			return fmt.Errorf("wgctrl: invalid preshared key: %w", err)
		}
		peerCfg.PresharedKey = &pskKey
	}

	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerCfg},
	}

	return m.client.ConfigureDevice(m.iface, cfg)
}

func (m *Manager) RemovePeer(pubkey string) error {
	key, err := wgtypes.ParseKey(pubkey)
	if err != nil {
		return fmt.Errorf("wgctrl: invalid public key %q: %w", pubkey, err)
	}

	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{{
			PublicKey: key,
			Remove:    true,
		}},
	}

	return m.client.ConfigureDevice(m.iface, cfg)
}

func (m *Manager) ListPeers() ([]PeerInfo, error) {
	dev, err := m.client.Device(m.iface)
	if err != nil {
		return nil, fmt.Errorf("wgctrl: failed to get device %q: %w", m.iface, err)
	}

	peers := make([]PeerInfo, 0, len(dev.Peers))
	for _, p := range dev.Peers {
		peers = append(peers, PeerInfo{
			PublicKey:  p.PublicKey.String(),
			AllowedIPs: p.AllowedIPs,
			RXBytes:    p.ReceiveBytes,
			TXBytes:    p.TransmitBytes,
		})
	}
	return peers, nil
}

func (m *Manager) GetDevice() (*wgtypes.Device, error) {
	dev, err := m.client.Device(m.iface)
	if err != nil {
		return nil, fmt.Errorf("wgctrl: failed to get device %q: %w", m.iface, err)
	}
	return dev, nil
}

func (m *Manager) Close() error {
	return m.client.Close()
}
