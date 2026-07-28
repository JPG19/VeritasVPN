package wireguard

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

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

// EnsureInterface brings up the WireGuard device with listen port, address, and private key.
func (m *Manager) EnsureInterface(privateKey wgtypes.Key, listenPort int, addressCIDR string) error {
	if err := ensureLink(m.iface); err != nil {
		return err
	}

	if addressCIDR != "" {
		if err := ensureAddress(m.iface, addressCIDR); err != nil {
			return err
		}
	}

	cfg := wgtypes.Config{
		PrivateKey: &privateKey,
		ListenPort: &listenPort,
	}
	if err := m.client.ConfigureDevice(m.iface, cfg); err != nil {
		return fmt.Errorf("wgctrl configure %s: %w", m.iface, err)
	}

	if err := exec.Command("ip", "link", "set", "up", "dev", m.iface).Run(); err != nil {
		return fmt.Errorf("ip link set up %s: %w", m.iface, err)
	}
	return nil
}

func ensureLink(iface string) error {
	if err := exec.Command("ip", "link", "show", "dev", iface).Run(); err == nil {
		return nil
	}
	if err := exec.Command("ip", "link", "add", "dev", iface, "type", "wireguard").Run(); err != nil {
		return fmt.Errorf("ip link add %s: %w", iface, err)
	}
	return nil
}

func ensureAddress(iface, addressCIDR string) error {
	out, err := exec.Command("ip", "-4", "addr", "show", "dev", iface).CombinedOutput()
	if err == nil && strings.Contains(string(out), strings.Split(addressCIDR, "/")[0]) {
		return nil
	}
	if err := exec.Command("ip", "addr", "add", addressCIDR, "dev", iface).Run(); err != nil {
		// Address may already exist with a different prefix; treat as non-fatal if present.
		out2, _ := exec.Command("ip", "-4", "addr", "show", "dev", iface).CombinedOutput()
		if strings.Contains(string(out2), strings.Split(addressCIDR, "/")[0]) {
			return nil
		}
		return fmt.Errorf("ip addr add %s: %w", addressCIDR, err)
	}
	return nil
}

func (m *Manager) AddPeer(pubkey string, allowedIPs []net.IPNet, psk *string) error {
	key, err := wgtypes.ParseKey(pubkey)
	if err != nil {
		return fmt.Errorf("wgctrl: invalid public key %q: %w", pubkey, err)
	}

	peerCfg := wgtypes.PeerConfig{
		PublicKey:         key,
		AllowedIPs:        allowedIPs,
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
