use base64::{engine::general_purpose::STANDARD, Engine};
use rand_core::OsRng;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use tauri::{AppHandle, Manager};
use x25519_dalek::{PublicKey, StaticSecret};

#[derive(Debug, Serialize, Deserialize)]
pub struct ProxyConfig {
    pub host: String,
    pub port: u16,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct WgTunnelConfig {
    pub private_key: String,
    pub address: String,
    pub dns: String,
    pub server_public_key: String,
    pub endpoint: String,
    pub allowed_ips: Vec<String>,
    pub peer_id: String,
    #[serde(default)]
    pub preshared_key: String,
}

#[derive(Debug, Serialize)]
pub struct ConnectResult {
    pub success: bool,
    pub message: String,
    pub mode: String,
    pub peer_id: String,
}

#[derive(Debug, Serialize)]
pub struct KeyPair {
    pub private_key: String,
    pub public_key: String,
}

fn state_dir() -> Result<PathBuf, String> {
    let home = dirs_next::home_dir().ok_or("Could not resolve home directory")?;
    #[cfg(target_os = "macos")]
    let dir = home
        .join("Library")
        .join("Application Support")
        .join("cloud.veritasvpn.desktop");
    #[cfg(not(target_os = "macos"))]
    let dir = home.join(".veritasvpn");
    fs::create_dir_all(&dir).map_err(|e| format!("create config dir: {e}"))?;
    Ok(dir)
}

fn conf_path() -> Result<PathBuf, String> {
    Ok(state_dir()?.join("veritas.conf"))
}

fn peer_id_path() -> Result<PathBuf, String> {
    Ok(state_dir()?.join("peer_id"))
}

fn iface_path() -> Result<PathBuf, String> {
    Ok(state_dir()?.join("iface"))
}

fn pid_path() -> Result<PathBuf, String> {
    Ok(state_dir()?.join("wireguard-go.pid"))
}

fn resolve_wireguard_go(app: &AppHandle) -> Result<PathBuf, String> {
    let candidates = [
        "bin/wireguard-go",
        "resources/bin/wireguard-go",
    ];
    for rel in candidates {
        if let Ok(p) = app
            .path()
            .resolve(rel, tauri::path::BaseDirectory::Resource)
        {
            if p.exists() {
                return Ok(p);
            }
        }
    }
    if let Ok(dir) = app.path().resource_dir() {
        for rel in [
            dir.join("bin/wireguard-go"),
            dir.join("resources/bin/wireguard-go"),
        ] {
            if rel.exists() {
                return Ok(rel);
            }
        }
    }
    let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("resources/bin/wireguard-go");
    if dev.exists() {
        return Ok(dev);
    }
    Err("Bundled WireGuard engine missing from the app".into())
}

#[tauri::command]
fn wireguard_available(app: AppHandle) -> bool {
    resolve_wireguard_go(&app).is_ok()
}

fn is_process_alive(pid: u32) -> bool {
    std::process::Command::new("kill")
        .args(["-0", &pid.to_string()])
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status()
        .map(|s| s.success())
        .unwrap_or(false)
}

fn cleanup_stale_state(dir: &std::path::Path) {
    let _ = std::fs::remove_file(dir.join("wireguard-go.pid"));
    let _ = std::fs::remove_file(dir.join("iface"));
    let _ = std::fs::remove_file(dir.join("iface.meta"));
}

#[tauri::command]
fn tunnel_status() -> bool {
    let dir = match state_dir() {
        Ok(d) => d,
        Err(_) => return false,
    };
    let pid_file = dir.join("wireguard-go.pid");
    let iface_file = dir.join("iface");
    if !pid_file.exists() || !iface_file.exists() {
        return false;
    }
    let pid = match std::fs::read_to_string(&pid_file) {
        Ok(s) => s.trim().parse::<u32>().unwrap_or(0),
        Err(_) => 0,
    };
    if pid == 0 || !is_process_alive(pid) {
        cleanup_stale_state(&dir);
        return false;
    }
    let iface = match std::fs::read_to_string(&iface_file) {
        Ok(s) => s.trim().to_string(),
        Err(_) => {
            cleanup_stale_state(&dir);
            return false;
        }
    };
    if iface.is_empty() {
        cleanup_stale_state(&dir);
        return false;
    }
    let ok = std::process::Command::new("ifconfig")
        .arg(&iface)
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status()
        .map(|s| s.success())
        .unwrap_or(false);
    if !ok {
        cleanup_stale_state(&dir);
        return false;
    }
    true
}

#[tauri::command]
fn saved_peer_id() -> String {
    peer_id_path().ok().and_then(|p| std::fs::read_to_string(p).ok()).unwrap_or_default().trim().to_string()
}

#[tauri::command]
fn clear_saved_state() {
    if let Ok(dir) = state_dir() {
        let _ = std::fs::remove_file(dir.join("peer_id"));
    }
}

#[tauri::command]
fn generate_wg_keys() -> Result<KeyPair, String> {
    let secret = StaticSecret::random_from_rng(OsRng);
    let public = PublicKey::from(&secret);
    Ok(KeyPair {
        private_key: STANDARD.encode(secret.to_bytes()),
        public_key: STANDARD.encode(public.to_bytes()),
    })
}

fn b64_key_to_hex(b64: &str) -> Result<String, String> {
    let bytes = STANDARD
        .decode(b64.trim())
        .map_err(|e| format!("invalid key: {e}"))?;
    if bytes.len() != 32 {
        return Err("WireGuard key must be 32 bytes".into());
    }
    Ok(hex::encode(bytes))
}

#[tauri::command]
async fn connect_wireguard(app: AppHandle, config: WgTunnelConfig) -> ConnectResult {
    let peer_id = config.peer_id.clone();
    let (tx, rx) = std::sync::mpsc::channel();
    std::thread::spawn(move || {
        let result = bring_up_wireguard(&app, &config);
        let _ = tx.send(result);
    });
    match rx.recv() {
        Ok(Ok(msg)) => ConnectResult {
            success: true,
            message: msg,
            mode: "wireguard".into(),
            peer_id,
        },
        Ok(Err(e)) => ConnectResult {
            success: false,
            message: e,
            mode: "wireguard".into(),
            peer_id,
        },
        Err(_) => ConnectResult {
            success: false,
            message: "internal error: connection handler panicked".into(),
            mode: "wireguard".into(),
            peer_id: String::new(),
        },
    }
}

#[tauri::command]
async fn disconnect_wireguard(app: AppHandle) -> ConnectResult {
    let app_clone = app.clone();
    let (tx, rx) = std::sync::mpsc::channel();
    std::thread::spawn(move || {
        let result = bring_down_wireguard(&app_clone);
        let _ = tx.send(result);
    });
    match rx.recv() {
        Ok(Ok(msg)) => ConnectResult {
            success: true,
            message: msg,
            mode: "wireguard".into(),
            peer_id: String::new(),
        },
        Ok(Err(e)) => ConnectResult {
            success: false,
            message: e,
            mode: "wireguard".into(),
            peer_id: String::new(),
        },
        Err(_) => ConnectResult {
            success: false,
            message: "internal error: disconnect handler panicked".into(),
            mode: "wireguard".into(),
            peer_id: String::new(),
        },
    }
}

fn bring_up_wireguard(app: &AppHandle, config: &WgTunnelConfig) -> Result<String, String> {
    let wg_go = resolve_wireguard_go(app)?;
    let dir = state_dir()?;
    let address = config
        .address
        .trim()
        .split('/')
        .next()
        .unwrap_or("")
        .to_string();
    if address.is_empty() {
        return Err("missing assigned address".into());
    }

    let priv_hex = b64_key_to_hex(&config.private_key)?;
    let pub_hex = b64_key_to_hex(&config.server_public_key)?;
    let endpoint = config.endpoint.trim().to_string();

    let allowed = if config.allowed_ips.is_empty() {
        vec!["0.0.0.0/0".into(), "::/0".into()]
    } else {
        config.allowed_ips.clone()
    };

    let mut uapi = format!(
        "set=1\nprivate_key={priv_hex}\nreplace_peers=true\npublic_key={pub_hex}\nendpoint={endpoint}\npersistent_keepalive_interval=25\n"
    );
    if !config.preshared_key.trim().is_empty() {
        let psk_hex = b64_key_to_hex(&config.preshared_key)?;
        uapi.push_str(&format!("preshared_key={psk_hex}\n"));
    }
    for ip in &allowed {
        uapi.push_str(&format!("allowed_ip={}\n", ip.trim()));
    }
    uapi.push('\n');

    let uapi_path = dir.join("uapi.txt");
    let script_path = dir.join("bringup.sh");
    let iface_file = iface_path()?;
    let pid_file = pid_path()?;

    fs::write(&uapi_path, &uapi).map_err(|e| format!("write uapi: {e}"))?;
    fs::write(
        conf_path()?,
        format!(
            "# VeritasVPN managed tunnel\n# endpoint {}\n# address {}\n",
            endpoint, config.address
        ),
    )
    .ok();
    fs::write(peer_id_path()?, config.peer_id.as_bytes()).ok();

    let script = build_bringup_script(
        &wg_go,
        &uapi_path,
        &iface_file,
        &pid_file,
        &address,
        if config.dns.trim().is_empty() {
            "1.1.1.1"
        } else {
            config.dns.trim()
        },
        &endpoint,
    );

    fs::write(&script_path, script).map_err(|e| format!("write script: {e}"))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = fs::metadata(&script_path)
            .map_err(|e| format!("stat script: {e}"))?
            .permissions();
        perms.set_mode(0o700);
        fs::set_permissions(&script_path, perms).ok();
    }

    run_elevated(&script_path)?;
    Ok(format!("WireGuard connected via {endpoint}"))
}

fn build_bringup_script(
    wg_go: &Path,
    uapi_path: &Path,
    iface_file: &Path,
    pid_file: &Path,
    address: &str,
    dns: &str,
    endpoint: &str,
) -> String {
    format!(
        r#"#!/bin/bash
set -uo pipefail
WG_GO='{wg_go}'
UAPI='{uapi}'
IFACE_FILE='{iface_file}'
PID_FILE='{pid_file}'
META_FILE='{iface_file}.meta'
ADDR='{address}'
DNS='{dns}'
ENDPOINT='{endpoint}'

# --- tear down any previous Veritas tunnel (best-effort) ---
if [[ -f "$PID_FILE" ]]; then
  kill "$(cat "$PID_FILE")" 2>/dev/null || true
  rm -f "$PID_FILE"
fi
if [[ -f "$IFACE_FILE" ]]; then
  OLD="$(cat "$IFACE_FILE")"
  route -n delete -net 0.0.0.0/1 -interface "$OLD" 2>/dev/null || true
  route -n delete -net 128.0.0.0/1 -interface "$OLD" 2>/dev/null || true
  ifconfig "$OLD" down 2>/dev/null || true
  rm -f "$IFACE_FILE"
fi
# Drop stale split-default routes even if iface file was lost
route -n delete -net 0.0.0.0/1 2>/dev/null || true
route -n delete -net 128.0.0.0/1 2>/dev/null || true
pkill -f '/wireguard-go utun' 2>/dev/null || true
rm -f /var/run/wireguard/*.sock 2>/dev/null || true

# Capture the REAL default gateway BEFORE we install tunnel routes.
# Without a host route to the WG endpoint via this gateway, 0.0.0.0/1
# blackholes WireGuard UDP itself and kills all internet.
GW="$(route -n get default 2>/dev/null | awk '/gateway: / {{print $2; exit}}')"
GW_IF="$(route -n get default 2>/dev/null | awk '/interface: / {{print $2; exit}}')"
ENDPOINT_HOST="${{ENDPOINT%%:*}}"
ENDPOINT_IP=""
if [[ -n "$ENDPOINT_HOST" ]]; then
  if [[ "$ENDPOINT_HOST" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    ENDPOINT_IP="$ENDPOINT_HOST"
  else
    ENDPOINT_IP="$(dscacheutil -q host -a name "$ENDPOINT_HOST" 2>/dev/null | awk '/ip_address: /{{print $2; exit}}')"
    if [[ -z "$ENDPOINT_IP" ]]; then
      ENDPOINT_IP="$(python3 -c "import socket; print(socket.gethostbyname('$ENDPOINT_HOST'))" 2>/dev/null || true)"
    fi
  fi
fi

"$WG_GO" utun >/tmp/veritas-wg-go.log 2>&1 &
echo $! > "$PID_FILE"
sleep 0.5

IFACE=""
for _ in $(seq 1 40); do
  for sock in /var/run/wireguard/*.sock; do
    [[ -e "$sock" ]] || continue
    IFACE="$(basename "$sock" .sock)"
    break 2
  done
  sleep 0.1
done
if [[ -z "$IFACE" ]]; then
  echo "failed to start WireGuard engine" >&2
  cat /tmp/veritas-wg-go.log >&2 || true
  exit 1
fi
echo "$IFACE" > "$IFACE_FILE"

python3 - "$IFACE" "$UAPI" <<'PY'
import socket, sys, pathlib
iface, uapi = sys.argv[1], sys.argv[2]
sock_path = f"/var/run/wireguard/{{iface}}.sock"
data = pathlib.Path(uapi).read_bytes()
s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect(sock_path)
s.sendall(data)
s.shutdown(socket.SHUT_WR)
resp = s.recv(4096).decode("utf-8", "replace")
s.close()
if "errno=0" not in resp:
    sys.stderr.write(resp + "\n")
    sys.exit(1)
PY

ifconfig "$IFACE" inet "$ADDR" "$ADDR" netmask 255.255.255.255 up

# Keep the VPN server reachable outside the tunnel.
if [[ -n "$ENDPOINT_IP" && -n "$GW" ]]; then
  route -n delete -host "$ENDPOINT_IP" 2>/dev/null || true
  route -n add -host "$ENDPOINT_IP" "$GW"
fi

# Verify the tunnel actually forwards traffic before rerouting everything.
for _ in $(seq 1 5); do
  if ping -c 1 -t 1 -S "$ADDR" "$DNS" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
if ! ping -c 1 -t 1 -S "$ADDR" "$DNS" >/dev/null 2>&1; then
  echo "tunnel not forwarding — tearing down" >&2
  kill "$(cat "$PID_FILE")" 2>/dev/null || true
  ifconfig "$IFACE" down 2>/dev/null || true
  rm -f "$IFACE_FILE" "$PID_FILE" "$META_FILE"
  exit 1
fi

# Split default (like wg-quick) so we don't replace the system default route entry.
route -n delete -net 0.0.0.0/1 -interface "$IFACE" 2>/dev/null || true
route -n delete -net 128.0.0.0/1 -interface "$IFACE" 2>/dev/null || true
route -n add -net 0.0.0.0/1 -interface "$IFACE"
route -n add -net 128.0.0.0/1 -interface "$IFACE"

# Verify tunnel forwards traffic NOW (after routes installed).
# If the server can't forward, we must tear down immediately before
# the user loses internet with no recovery path.
for _ in $(seq 1 5); do
  if ping -c 1 -t 1 "$DNS" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
if ! ping -c 1 -t 1 "$DNS" >/dev/null 2>&1; then
  echo "tunnel not forwarding after routes — tearing down" >&2
  route -n delete -net 0.0.0.0/1 -interface "$IFACE" 2>/dev/null || true
  route -n delete -net 128.0.0.0/1 -interface "$IFACE" 2>/dev/null || true
  route -n delete -net 0.0.0.0/1 2>/dev/null || true
  route -n delete -net 128.0.0.0/1 2>/dev/null || true
  if [[ -n "$ENDPOINT_IP" ]]; then
    route -n delete -host "$ENDPOINT_IP" 2>/dev/null || true
  fi
  kill "$(cat "$PID_FILE")" 2>/dev/null || true
  ifconfig "$IFACE" down 2>/dev/null || true
  rm -f "$IFACE_FILE" "$PID_FILE" "$META_FILE"
  exit 1
fi

# Active network service for DNS (not "first listed").
SERVICE="$(networksetup -listnetworkserviceorder 2>/dev/null | awk -v iface="$GW_IF" '
  /^\(Hardware Port:/ {{
    name=$0
    sub(/^\(Hardware Port: /, "", name)
    sub(/,.*/, "", name)
  }}
  /Device: / {{
    dev=$2
    sub(/\).*/, "", dev)
    if (iface != "" && dev == iface) {{ print name; exit }}
  }}
')"
if [[ -z "$SERVICE" ]]; then
  SERVICE="$(networksetup -listallnetworkservices 2>/dev/null | awk 'NR==2{{print; exit}}')"
fi
if [[ -z "$SERVICE" ]]; then SERVICE="Wi-Fi"; fi
networksetup -setdnsservers "$SERVICE" "$DNS" 2>/dev/null || true

# Persist enough state for a reliable teardown even if the app crashes.
printf 'endpoint_ip=%s\ngateway=%s\nservice=%s\niface=%s\n' \
  "$ENDPOINT_IP" "$GW" "$SERVICE" "$IFACE" > "$META_FILE"

echo "ok iface=$IFACE endpoint_ip=$ENDPOINT_IP gw=$GW"
"#,
        wg_go = wg_go.display(),
        uapi = uapi_path.display(),
        iface_file = iface_file.display(),
        pid_file = pid_file.display(),
        address = address,
        dns = dns,
        endpoint = endpoint,
    )
}

fn bring_down_wireguard(_app: &AppHandle) -> Result<String, String> {
    let script_path = state_dir()?.join("teardown.sh");
    let iface_file = iface_path()?;
    let pid_file = pid_path()?;
    let meta_file = state_dir()?.join("iface.meta");
    // Never use `set -e` here — partial cleanup must still complete.
    let script = format!(
        r#"#!/bin/bash
set -uo pipefail
IFACE_FILE='{iface_file}'
PID_FILE='{pid_file}'
META_FILE='{meta_file}'

ENDPOINT_IP=""
GW=""
SERVICE=""
IFACE=""

if [[ -f "$META_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$META_FILE" 2>/dev/null || true
  ENDPOINT_IP="${{endpoint_ip:-}}"
  GW="${{gateway:-}}"
  SERVICE="${{service:-}}"
  IFACE="${{iface:-}}"
fi
if [[ -z "$IFACE" && -f "$IFACE_FILE" ]]; then
  IFACE="$(cat "$IFACE_FILE")"
fi

# Remove full-tunnel split routes (by iface and globally).
if [[ -n "$IFACE" ]]; then
  route -n delete -net 0.0.0.0/1 -interface "$IFACE" 2>/dev/null || true
  route -n delete -net 128.0.0.0/1 -interface "$IFACE" 2>/dev/null || true
  ifconfig "$IFACE" down 2>/dev/null || true
fi
route -n delete -net 0.0.0.0/1 2>/dev/null || true
route -n delete -net 128.0.0.0/1 2>/dev/null || true

# Remove pinned endpoint host route.
if [[ -n "$ENDPOINT_IP" ]]; then
  route -n delete -host "$ENDPOINT_IP" 2>/dev/null || true
fi

# Stop userspace WireGuard.
if [[ -f "$PID_FILE" ]]; then
  kill "$(cat "$PID_FILE")" 2>/dev/null || true
  sleep 0.2
  kill -9 "$(cat "$PID_FILE")" 2>/dev/null || true
  rm -f "$PID_FILE"
fi
pkill -f '/wireguard-go utun' 2>/dev/null || true
rm -f /var/run/wireguard/*.sock 2>/dev/null || true
rm -f "$IFACE_FILE" "$META_FILE"

# Restore DNS on the service we changed, else try common names.
if [[ -z "$SERVICE" ]]; then
  SERVICE="$(networksetup -listallnetworkservices 2>/dev/null | awk 'NR==2{{print; exit}}')"
fi
if [[ -n "$SERVICE" ]]; then
  networksetup -setdnsservers "$SERVICE" Empty 2>/dev/null || true
fi
for S in "Wi-Fi" "Ethernet" "Thunderbolt Ethernet" "USB 10/100/1000 LAN"; do
  networksetup -setdnsservers "$S" Empty 2>/dev/null || true
done

echo ok
"#,
        iface_file = iface_file.display(),
        pid_file = pid_file.display(),
        meta_file = meta_file.display(),
    );
    fs::write(&script_path, script).map_err(|e| format!("write teardown: {e}"))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = fs::metadata(&script_path)
            .map_err(|e| format!("stat teardown: {e}"))?
            .permissions();
        perms.set_mode(0o700);
        fs::set_permissions(&script_path, perms).ok();
    }
    // Prefer elevated teardown; if the user cancels the password prompt, still
    // try a best-effort non-elevated cleanup so we don't leave them offline.
    if let Err(elev_err) = run_elevated(&script_path) {
        let _ = Command::new("bash").arg(&script_path).output();
        let _ = fs::remove_file(conf_path()?);
        let _ = fs::remove_file(peer_id_path()?);
        return Err(format!(
            "disconnect needs admin rights to fully restore networking: {elev_err}"
        ));
    }
    let _ = fs::remove_file(conf_path()?);
    let _ = fs::remove_file(peer_id_path()?);
    Ok("WireGuard disconnected".into())
}

fn run_elevated(script: &Path) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        let path = script
            .to_string_lossy()
            .replace('\\', "\\\\")
            .replace('"', "\\\"");
        let apple = format!(r#"do shell script "bash \"{path}\"" with administrator privileges"#);
        let output = Command::new("osascript")
            .args(["-e", &apple])
            .output()
            .map_err(|e| format!("osascript: {e}"))?;
        if !output.status.success() {
            let err = String::from_utf8_lossy(&output.stderr);
            let out = String::from_utf8_lossy(&output.stdout);
            return Err(format!("privilege bring-up failed: {err} {out}"));
        }
        return Ok(());
    }
    #[cfg(not(target_os = "macos"))]
    {
        let output = Command::new("bash")
            .arg(script)
            .output()
            .map_err(|e| format!("bash: {e}"))?;
        if !output.status.success() {
            return Err(format!(
                "bring-up failed: {}",
                String::from_utf8_lossy(&output.stderr)
            ));
        }
        Ok(())
    }
}

#[tauri::command]
fn connect_socks(config: ProxyConfig) -> ConnectResult {
    match set_system_proxy(&config.host, config.port) {
        Ok(msg) => ConnectResult {
            success: true,
            message: msg,
            mode: "socks".into(),
            peer_id: String::new(),
        },
        Err(e) => ConnectResult {
            success: false,
            message: e,
            mode: "socks".into(),
            peer_id: String::new(),
        },
    }
}

#[tauri::command]
fn disconnect_socks() -> ConnectResult {
    match remove_system_proxy() {
        Ok(msg) => ConnectResult {
            success: true,
            message: msg,
            mode: "socks".into(),
            peer_id: String::new(),
        },
        Err(e) => ConnectResult {
            success: false,
            message: e,
            mode: "socks".into(),
            peer_id: String::new(),
        },
    }
}

fn set_system_proxy(host: &str, port: u16) -> Result<String, String> {
    #[cfg(target_os = "macos")]
    {
        return set_proxy_macos(host, port);
    }
    #[cfg(target_os = "windows")]
    {
        return set_proxy_windows(host, port);
    }
    #[cfg(target_os = "linux")]
    {
        return set_proxy_linux(host, port);
    }
    #[allow(unreachable_code)]
    Err("Unsupported platform".into())
}

fn remove_system_proxy() -> Result<String, String> {
    #[cfg(target_os = "macos")]
    {
        return remove_proxy_macos();
    }
    #[cfg(target_os = "windows")]
    {
        return remove_proxy_windows();
    }
    #[cfg(target_os = "linux")]
    {
        return remove_proxy_linux();
    }
    #[allow(unreachable_code)]
    Err("Unsupported platform".into())
}

#[cfg(target_os = "macos")]
fn get_active_network_service() -> Result<String, String> {
    let output = Command::new("sh")
        .arg("-c")
        .arg("networksetup -listnetworkserviceorder | grep -B1 \"$(route -n get default 2>/dev/null | grep interface | awk '{print $2}')\" | head -1 | sed 's/^([0-9]*) //'")
        .output()
        .map_err(|e| format!("Failed to detect network service: {}", e))?;

    let service = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if service.is_empty() {
        for candidate in &["Wi-Fi", "Ethernet", "USB 10/100/1000 LAN"] {
            let check = Command::new("networksetup")
                .args(["-getinfo", candidate])
                .output();
            if check.is_ok() {
                return Ok(candidate.to_string());
            }
        }
        return Err("Could not detect active network service".into());
    }
    Ok(service)
}

#[cfg(target_os = "macos")]
fn set_proxy_macos(host: &str, port: u16) -> Result<String, String> {
    let service = get_active_network_service()?;
    let port_str = port.to_string();
    Command::new("networksetup")
        .args(["-setsocksfirewallproxy", &service, host, &port_str])
        .output()
        .map_err(|e| format!("Failed to set SOCKS proxy: {}", e))?;
    Command::new("networksetup")
        .args(["-setsocksfirewallproxystate", &service, "on"])
        .output()
        .map_err(|e| format!("Failed to enable SOCKS proxy: {}", e))?;
    Ok(format!("SOCKS5 proxy set on {} -> {}:{}", service, host, port))
}

#[cfg(target_os = "macos")]
fn remove_proxy_macos() -> Result<String, String> {
    let service = get_active_network_service()?;
    Command::new("networksetup")
        .args(["-setsocksfirewallproxystate", &service, "off"])
        .output()
        .map_err(|e| format!("Failed to disable SOCKS proxy: {}", e))?;
    Ok(format!("SOCKS5 proxy disabled on {}", service))
}

#[cfg(target_os = "windows")]
fn set_proxy_windows(host: &str, port: u16) -> Result<String, String> {
    use winreg::enums::*;
    use winreg::RegKey;
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let proxy_path = "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings";
    let settings = hkcu
        .open_subkey_with_flags(proxy_path, KEY_SET_VALUE)
        .map_err(|e| format!("Failed to open registry: {}", e))?;
    let proxy_addr = format!("{}:{}", host, port);
    settings
        .set_value("ProxyServer", &proxy_addr)
        .map_err(|e| format!("Failed to set ProxyServer: {}", e))?;
    settings
        .set_value("ProxyEnable", &1u32)
        .map_err(|e| format!("Failed to enable proxy: {}", e))?;
    Ok(format!("SOCKS5 proxy set -> {}:{}", host, port))
}

#[cfg(target_os = "windows")]
fn remove_proxy_windows() -> Result<String, String> {
    use winreg::enums::*;
    use winreg::RegKey;
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let proxy_path = "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings";
    let settings = hkcu
        .open_subkey_with_flags(proxy_path, KEY_SET_VALUE)
        .map_err(|e| format!("Failed to open registry: {}", e))?;
    settings
        .set_value("ProxyEnable", &0u32)
        .map_err(|e| format!("Failed to disable proxy: {}", e))?;
    Ok("System proxy disabled".into())
}

#[cfg(target_os = "linux")]
fn set_proxy_linux(host: &str, port: u16) -> Result<String, String> {
    let port_str = port.to_string();
    let _ = Command::new("gsettings")
        .args(["set", "org.gnome.system.proxy", "mode", "'manual'"])
        .output();
    let _ = Command::new("gsettings")
        .args([
            "set",
            "org.gnome.system.proxy.socks",
            "host",
            &format!("'{}'", host),
        ])
        .output();
    let _ = Command::new("gsettings")
        .args(["set", "org.gnome.system.proxy.socks", "port", &port_str])
        .output();
    Ok(format!("SOCKS5 proxy set (GNOME) -> {}:{}", host, port))
}

#[cfg(target_os = "linux")]
fn remove_proxy_linux() -> Result<String, String> {
    let _ = Command::new("gsettings")
        .args(["set", "org.gnome.system.proxy", "mode", "'none'"])
        .output();
    Ok("System proxy disabled (GNOME)".into())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_http::init())
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            wireguard_available,
            tunnel_status,
            saved_peer_id,
            clear_saved_state,
            generate_wg_keys,
            connect_wireguard,
            disconnect_wireguard,
            connect_socks,
            disconnect_socks
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
