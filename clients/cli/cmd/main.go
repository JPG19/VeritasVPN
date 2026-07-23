package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const apiBaseURL = "https://api.veritasvpn.com/api/v1"

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
  veritas connect [--region <region>] Connect to VPN (default: auto-select)
  veritas disconnect                  Disconnect from VPN
  veritas account                     Show account details
  veritas help                        Show this help

Environment variables:
  VERITAS_ACCOUNT_ID   Your account ID
  VERITAS_ACCESS_TOKEN  Your access token
  VERITAS_API_URL       API base URL (default: https://api.veritasvpn.com)
`)
}

func cmdRegister() {
	fmt.Println("Registering new account...")
	fmt.Println("No email required. Save your account ID and token.")

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
	}
	json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("\nIMPORTANT — Save these credentials:\n")
	fmt.Printf("Account ID:    %s\n", result.AccountID)
	fmt.Printf("Access Token:  %s\n", result.AccessToken)
	fmt.Printf("Refresh Token: %s\n", result.RefreshToken)
	fmt.Printf("\nSet environment variables:\n")
	fmt.Printf("  export VERITAS_ACCOUNT_ID=%s\n", result.AccountID)
	fmt.Printf("  export VERITAS_ACCESS_TOKEN=%s\n", result.AccessToken)
}

func cmdListServers() {
	resp, err := apiGet("/servers")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list servers: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Servers []struct {
			ID         string  `json:"id"`
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

	fmt.Printf("Connecting...")
	if region != "" {
		fmt.Printf(" (region: %s)", region)
	}
	fmt.Println()

	accountID := os.Getenv("VERITAS_ACCOUNT_ID")
	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")
	if accountID == "" || accessToken == "" {
		fmt.Fprintf(os.Stderr, "set VERITAS_ACCOUNT_ID and VERITAS_ACCESS_TOKEN\n")
		os.Exit(1)
	}

	body, _ := json.Marshal(map[string]string{
		"region": region,
	})

	resp, err := apiPostWithAuth("/wg/peers", body, accessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		PeerID         string   `json:"peer_id"`
		ServerHostname string   `json:"server_hostname"`
		ServerEndpoint string   `json:"server_endpoint"`
		ServerPubkey   string   `json:"server_public_key"`
		AssignedIP     string   `json:"assigned_ip"`
		DNSServer      string   `json:"dns_server"`
		AllowedIPs     []string `json:"allowed_ips"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("\nConnected to %s (%s)\n", result.ServerHostname, result.ServerEndpoint)
	fmt.Printf("Assigned IP: %s\n", result.AssignedIP)
	fmt.Printf("DNS: %s\n", result.DNSServer)

	config := fmt.Sprintf(`[Interface]
PrivateKey = <YOUR_PRIVATE_KEY>
Address = %s
DNS = %s

[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = 25
`, result.AssignedIP, result.DNSServer, result.ServerPubkey,
		strings.Join(result.AllowedIPs, ", "), result.ServerEndpoint)

	configPath := os.Getenv("HOME") + "/.veritasvpn/wg.conf"
	os.MkdirAll(os.Getenv("HOME")+"/.veritasvpn", 0700)
	os.WriteFile(configPath, []byte(config), 0600)

	fmt.Printf("\nConfig saved to %s\n", configPath)
	fmt.Println("Run: wg-quick up ~/.veritasvpn/wg.conf")
}

func cmdDisconnect() {
	fmt.Println("Disconnecting...")
	os.Remove(os.Getenv("HOME") + "/.veritasvpn/wg.conf")
	fmt.Println("Run: wg-quick down ~/.veritasvpn/wg.conf")
}

func cmdStatus() {
	accountID := os.Getenv("VERITAS_ACCOUNT_ID")
	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")

	resp, err := apiGetWithAuth("/wg/peers", accessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Peers []struct {
			PeerID         string `json:"peer_id"`
			ServerHostname string `json:"server_hostname"`
			AssignedIP     string `json:"assigned_ip"`
			Status         string `json:"status"`
		} `json:"peers"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	_ = accountID
	fmt.Printf("Active peers: %d\n", len(result.Peers))
	for _, p := range result.Peers {
		fmt.Printf("  %s → %s (IP: %s, status: %s)\n",
			p.PeerID, p.ServerHostname, p.AssignedIP, p.Status)
	}
}

func cmdAccount() {
	accountID := os.Getenv("VERITAS_ACCOUNT_ID")
	accessToken := os.Getenv("VERITAS_ACCESS_TOKEN")

	resp, err := apiGetWithAuth("/auth/account?account_id="+accountID, accessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "account lookup failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		AccountID          string `json:"account_id"`
		Tier               string `json:"tier"`
		Status             string `json:"status"`
		SubscriptionExpiry int64  `json:"subscription_expiry,omitempty"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("Account ID: %s\n", result.AccountID)
	fmt.Printf("Tier:       %s\n", result.Tier)
	fmt.Printf("Status:     %s\n", result.Status)
	if result.SubscriptionExpiry > 0 {
		exp := time.Unix(result.SubscriptionExpiry, 0)
		fmt.Printf("Expires:    %s\n", exp.Format(time.RFC3339))
	}
}

func apiGet(path string) (*http.Response, error) {
	url := apiBaseURL + path
	req, _ := http.NewRequest("GET", url, nil)
	return client.Do(req)
}

func apiGetWithAuth(path, token string) (*http.Response, error) {
	url := apiBaseURL + path
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return client.Do(req)
}

func apiPost(path string, body interface{}) (*http.Response, error) {
	url := apiBaseURL + path
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(jsonBody)))
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func apiPostWithAuth(path string, body []byte, token string) (*http.Response, error) {
	url := apiBaseURL + path
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return client.Do(req)
}
