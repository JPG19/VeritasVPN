package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func apiBase() string {
	if v := os.Getenv("VERITAS_API_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://veritasvpn.cloud/api/v1"
}

var client = &http.Client{Timeout: 15 * time.Second}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "register":
		cmdRegister()
	case "status":
		cmdStatus()
	case "servers":
		cmdListServers()
	case "connect":
		cmdConnect()
	case "disconnect":
		cmdDisconnect()
	case "account":
		cmdAccount()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`veritas — VeritasVPN CLI client

Usage:
  veritas register                    Register a new account
  veritas status                      Show connection status
  veritas servers                     List available servers
  veritas connect [--region <region>] Connect via WireGuard
  veritas disconnect                  Disconnect WireGuard
  veritas account                     Show account details
  veritas help                        Show this help

Environment variables:
  VERITAS_ACCOUNT_ID    Your account ID
  VERITAS_ACCESS_TOKEN  Your access token
  VERITAS_API_URL       API base URL (default: https://veritasvpn.cloud/api/v1)
`)
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".veritasvpn")
}

func confPath() string {
	return filepath.Join(configDir(), "veritas.conf")
}

func peerIDPath() string {
	return filepath.Join(configDir(), "peer_id")
}

func generateWGKeys() (priv, pub string, err error) {
	out, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", "", fmt.Errorf("wg genkey (install wireguard-tools): %w", err)
	}
	priv = strings.TrimSpace(string(out))
	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = strings.NewReader(priv)
	out, err = cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("wg pubkey: %w", err)
	}
	pub = strings.TrimSpace(string(out))
	return priv, pub, nil
}

func cmdRegister() {
	fmt.Println("Registering new account...")

	resp, err := apiPost("/auth/register", map[string]string{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "registration failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		AccountID    string `json:"account_id"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.AccountID == "" {
		fmt.Fprintf(os.Stderr, "registration failed: %s\n", result.Error)
		os.Exit(1)
	}

	fmt.Printf("\nIMPORTANT — Save these credentials:\n")
	fmt.Printf("Account ID:    %s\n", result.AccountID)
	fmt.Printf("Access Token:  %s\n", result.AccessToken)
	fmt.Printf("Refresh Token: %s\n", result.RefreshToken)
	fmt.Printf("\nSet environment variables:\n")
	fmt.Printf("  export VERITAS_ACCOUNT_ID=%s\n", result.AccountID)
	fmt.Printf("  export VERITAS_ACCESS_TOKEN=%s\n", result.AccessToken)
}

func cmdListServers() {
	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")
	resp, err := apiGetWithAuth("/wg/servers", accessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list servers: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Servers []struct {
			Hostname   string  `json:"hostname"`
			Region     string  `json:"region"`
			City       string  `json:"city"`
			Country    string  `json:"country"`
			LoadFactor float64 `json:"load_factor"`
			Status     string  `json:"status"`
		} `json:"servers"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("%-20s %-15s %-15s %-10s %-8s\n", "HOSTNAME", "CITY", "COUNTRY", "LOAD %", "STATUS")
	fmt.Println(strings.Repeat("-", 72))
	for _, s := range result.Servers {
		fmt.Printf("%-20s %-15s %-15s %-8.1f %-8s\n",
			s.Hostname, s.City, s.Country, s.LoadFactor*100, s.Status)
	}
}

func cmdConnect() {
	region := ""
	for i, arg := range os.Args {
		if arg == "--region" && i+1 < len(os.Args) {
			region = os.Args[i+1]
		}
	}

	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")
	if accessToken == "" {
		fmt.Fprintf(os.Stderr, "set VERITAS_ACCESS_TOKEN\n")
		os.Exit(1)
	}

	priv, pub, err := generateWGKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	body, _ := json.Marshal(map[string]string{
		"public_key": pub,
		"region":     region,
	})

	resp, err := apiPostWithAuth("/wg/peers", body, accessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		PeerID           string   `json:"peer_id"`
		ServerHostname   string   `json:"server_hostname"`
		ServerEndpoint   string   `json:"server_endpoint"`
		ServerPubkey     string   `json:"server_public_key"`
		AssignedIP       string   `json:"assigned_ip"`
		DNSServer        string   `json:"dns_server"`
		PresharedKey     string   `json:"preshared_key"`
		AllowedIPs       []string `json:"allowed_ips"`
		ClientAllowedIPs []string `json:"client_allowed_ips"`
		Error            string   `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.PeerID == "" {
		fmt.Fprintf(os.Stderr, "connect failed: %s\n", result.Error)
		os.Exit(1)
	}

	allowed := result.ClientAllowedIPs
	if len(allowed) == 0 {
		allowed = result.AllowedIPs
	}
	if len(allowed) == 0 {
		allowed = []string{"0.0.0.0/0", "::/0"}
	}
	dns := result.DNSServer
	if dns == "" {
		dns = "1.1.1.1"
	}

	pskLine := ""
	if result.PresharedKey != "" {
		pskLine = fmt.Sprintf("PresharedKey = %s\n", result.PresharedKey)
	}

	config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
DNS = %s

[Peer]
PublicKey = %s
%sAllowedIPs = %s
Endpoint = %s
PersistentKeepalive = 25
`, priv, result.AssignedIP, dns, result.ServerPubkey, pskLine,
		strings.Join(allowed, ", "), result.ServerEndpoint)

	_ = os.MkdirAll(configDir(), 0700)
	_ = os.WriteFile(confPath(), []byte(config), 0600)
	_ = os.WriteFile(peerIDPath(), []byte(result.PeerID), 0600)

	fmt.Printf("Connected to %s (%s)\n", result.ServerHostname, result.ServerEndpoint)
	fmt.Printf("Assigned IP: %s\n", result.AssignedIP)
	fmt.Printf("Config: %s\n", confPath())

	if path, err := exec.LookPath("wg-quick"); err == nil {
		cmd := exec.Command(path, "up", confPath())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "wg-quick up failed (run manually): %v\n", err)
		}
	} else {
		fmt.Println("Run: wg-quick up ~/.veritasvpn/veritas.conf")
	}
}

func cmdDisconnect() {
	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")
	if data, err := os.ReadFile(peerIDPath()); err == nil && accessToken != "" {
		peerID := strings.TrimSpace(string(data))
		req, _ := http.NewRequest("DELETE", apiBase()+"/wg/peers/"+peerID, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}

	if path, err := exec.LookPath("wg-quick"); err == nil {
		if _, err := os.Stat(confPath()); err == nil {
			cmd := exec.Command(path, "down", confPath())
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
	}

	_ = os.Remove(confPath())
	_ = os.Remove(peerIDPath())
	fmt.Println("Disconnected")
}

func cmdStatus() {
	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")
	resp, err := apiGetWithAuth("/wg/peers", accessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Peers []struct {
			ID         string `json:"id"`
			AssignedIP string `json:"assigned_ip"`
			Status     string `json:"status"`
		} `json:"peers"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("Active peers: %d\n", len(result.Peers))
	for _, p := range result.Peers {
		fmt.Printf("  %s (IP: %s, status: %s)\n", p.ID, p.AssignedIP, p.Status)
	}
}

func cmdAccount() {
	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")
	resp, err := apiGetWithAuth("/auth/me", accessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "account lookup failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Account: %v\n", result)
}

func apiGetWithAuth(path, token string) (*http.Response, error) {
	url := apiBase() + path
	req, _ := http.NewRequest("GET", url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return client.Do(req)
}

func apiPost(path string, body interface{}) (*http.Response, error) {
	url := apiBase() + path
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(jsonBody)))
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func apiPostWithAuth(path string, body []byte, token string) (*http.Response, error) {
	url := apiBase() + path
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return client.Do(req)
}
