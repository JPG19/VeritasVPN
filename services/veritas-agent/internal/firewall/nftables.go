package firewall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const cmdTimeout = 10 * time.Second

type Manager struct {
	tableName string
}

func New() *Manager {
	return &Manager{tableName: "veritas"}
}

func (m *Manager) run(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nft", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("nft %s: timed out after %v", strings.Join(args, " "), cmdTimeout)
		}
		return fmt.Errorf("nft %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (m *Manager) runIgnore(args ...string) {
	_ = m.run(args...)
}

func (m *Manager) ensureTable() {
	m.runIgnore("add", "table", "inet", m.tableName)
}

func (m *Manager) ensureChain(chain, spec string) {
	m.runIgnore("add", "chain", "inet", m.tableName, chain, spec)
}

func (m *Manager) SetupNAT(iface string) error {
	egress := os.Getenv("EGRESS_IFACE")
	if egress == "" {
		return nil
	}
	m.ensureTable()
	m.ensureChain("nat", "{ type nat hook postrouting priority srcnat; }")
	return m.run("add", "rule", "inet", m.tableName, "nat", "oifname", egress, "masquerade")
}

func (m *Manager) SetupKillSwitch(wgIface string, wgPort int) error {
	m.ensureTable()
	m.ensureChain("forward", "{ type filter hook forward priority filter; policy accept; }")
	m.ensureChain("input", "{ type filter hook input priority filter; policy accept; }")

	m.runIgnore("add", "rule", "inet", m.tableName, "input", "ct", "state", "established,related", "accept")
	m.runIgnore("add", "rule", "inet", m.tableName, "forward", "ct", "state", "established,related", "accept")
	m.runIgnore("add", "rule", "inet", m.tableName, "input", "iifname", wgIface, "accept")
	m.runIgnore("add", "rule", "inet", m.tableName, "input", "udp", "dport", fmt.Sprintf("%d", wgPort), "accept")

	return nil
}

func (m *Manager) EnableKillSwitch() error {
	m.runIgnore("flush", "chain", "inet", m.tableName, "forward")

	m.runIgnore("add", "rule", "inet", m.tableName, "forward", "ct", "state", "established,related", "accept")
	m.runIgnore("add", "rule", "inet", m.tableName, "forward", "iifname", "wg0", "accept")
	return m.run("add", "rule", "inet", m.tableName, "forward", "drop")
}

func (m *Manager) DisableKillSwitch() error {
	m.runIgnore("flush", "chain", "inet", m.tableName, "forward")
	return m.run("add", "rule", "inet", m.tableName, "forward", "ct", "state", "established,related", "accept")
}

func (m *Manager) Cleanup() error {
	return m.run("delete", "table", "inet", m.tableName)
}
