#!/usr/bin/env bash
# Verify WireGuard UDP 51820 is reachable from the internet.
set -euo pipefail

PUBLIC_IP="${1:-}"
if [[ -z "$PUBLIC_IP" ]]; then
  PUBLIC_IP="${PUBLIC_IP:-$(curl -s --max-time 5 https://api.ipify.org 2>/dev/null || echo '')}"
fi

if [[ -z "$PUBLIC_IP" ]]; then
  echo "Usage: $0 <PUBLIC_IP>" >&2
  echo "  or set PUBLIC_IP env var." >&2
  exit 1
fi

WG_PORT="${WG_PORT:-51820}"
_TIMEOUT=3

echo "=== VeritasVPN Port Verification ==="
echo "  Public IP:  $PUBLIC_IP"
echo "  WG Port:    $WG_PORT/udp"
echo ""

# Check if port is listening locally
echo "[1/3] Local listener check..."
if command -v ss >/dev/null; then
  if ss -tuln | grep -q ":$WG_PORT "; then
    echo "  [ OK ] Port $WG_PORT is listening locally"
  else
    echo "  [WARN] Port $WG_PORT not found in local listeners"
  fi
elif command -v netstat >/dev/null; then
  if netstat -tuln | grep -q ":$WG_PORT "; then
    echo "  [ OK ] Port $WG_PORT is listening locally"
  else
    echo "  [WARN] Port $WG_PORT not found in local listeners"
  fi
else
  echo "  [SKIP] No ss/netstat available"
fi
echo ""

# Check if port is reachable from localhost
echo "[2/3] Localhost connectivity..."
if timeout $_TIMEOUT bash -c "echo >/dev/udp/127.0.0.1/$WG_PORT" 2>/dev/null; then
  echo "  [ OK ] Port $WG_PORT reachable on localhost"
else
  echo "  [WARN] Cannot reach port $WG_PORT on localhost"
fi
echo ""

# Check external reachability via public check services
echo "[3/3] External reachability check..."
echo "  (This tests if your router forwards UDP $WG_PORT to this host)"
echo ""

# Method 1: Use canyouseeme.org-style check (HTTP-based)
echo "  Trying portchecker.co..."
REACHABLE=$(curl -s --max-time 10 "https://api.portchecker.co/check?host=${PUBLIC_IP}&port=${WG_PORT}" 2>/dev/null || echo '')
if echo "$REACHABLE" | grep -qi '"reachable":\s*true'; then
  echo "  [ OK ] Port $WG_PORT appears reachable from internet"
  exit 0
fi

# Method 2: Use yougetsignal API
echo "  Trying yougetsignal..."
RESULT=$(curl -s --max-time 10 "https://ports.yougetsignal.com/check-port.php" \
  --data-raw "remoteAddress=${PUBLIC_IP}&portNumber=${WG_PORT}&Submit=Check" 2>/dev/null || echo '')
if echo "$RESULT" | grep -qi '"open"\|open'; then
  echo "  [ OK ] Port $WG_PORT appears reachable from internet"
  exit 0
fi

# Method 3: Local nmap-style check
if command -v nc >/dev/null; then
  echo "  Trying nmap online (portchecker)..."
  NMAP_RESULT=$(curl -s --max-time 10 "https://api.portchecker.co/check/${PUBLIC_IP}/${WG_PORT}" 2>/dev/null || echo '')
  if echo "$NMAP_RESULT" | grep -qi "true\|open"; then
    echo "  [ OK ] Port $WG_PORT appears reachable from internet"
    exit 0
  fi
fi

echo ""
echo "  [UNCERTAIN] Could not confirm external reachability automatically."
echo "  Verify manually:"
echo "    1. Check router forwards UDP $WG_PORT to $PUBLIC_IP"
echo "    2. Test from an external host: nc -uvz $PUBLIC_IP $WG_PORT"
echo "    3. Use https://portchecker.co with IP $PUBLIC_IP port $WG_PORT"
echo ""
echo "  If you're behind CGNAT, move the node to a VPS with a public IP."
exit 1
