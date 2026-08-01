import { useState, useEffect, FormEvent, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { fetch } from "@tauri-apps/plugin-http";
import {
  getStoredToken,
  refreshSession,
  signIn as doSignIn,
  signUp as doSignUp,
  signInWithAccountId as doSignInAccountId,
  registerAnonymous as doRegisterAnonymous,
  downloadAccountFile,
  signOut as doSignOut,
  type User,
} from "./auth";
import { AUTH_API, DEFAULT_PROXY } from "./config";
import "./App.css";

type AuthMode = "signin" | "signup";
/** Email/password vs Account ID (anonymous) path. */
type AuthMethod = "email" | "accountId";
type TunnelMode = "wireguard" | "socks" | "";

interface ConnectResult {
  success: boolean;
  message: string;
  mode: string;
  peer_id: string;
}

interface KeyPair {
  private_key: string;
  public_key: string;
}

interface PeerResponse {
  peer_id: string;
  server_public_key: string;
  server_endpoint: string;
  assigned_ip: string;
  dns_server: string;
  preshared_key?: string;
  allowed_ips?: string[];
  client_allowed_ips?: string[];
  error?: string;
}

function App() {
  const [user, setUser] = useState<User | null>(null);
  const [mode, setMode] = useState<AuthMode>("signin");
  const [method, setMethod] = useState<AuthMethod>("email");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [accountId, setAccountId] = useState("");
  const [newAccountId, setNewAccountId] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [tunnelMode, setTunnelMode] = useState<TunnelMode>("");
  const [peerId, setPeerId] = useState("");
  const [statusMsg, setStatusMsg] = useState("");
  const [disconnecting, setDisconnecting] = useState(false);

  useEffect(() => {
    if (!statusMsg) return;
    const t = setTimeout(() => setStatusMsg(""), 5000);
    return () => clearTimeout(t);
  }, [statusMsg]);

  useEffect(() => {
    invoke<boolean>("tunnel_status").then((up) => {
      if (up) {
        invoke<string>("saved_peer_id").then((pid) => {
          setConnected(true);
          setTunnelMode("wireguard");
          setPeerId(pid);
        });
      }
    });
  }, []);

  const switchMode = useCallback((next: AuthMode) => {
    setMode(next);
    setMethod("email");
    setError("");
    setNewAccountId("");
  }, []);

  const switchMethod = useCallback((next: AuthMethod) => {
    setMethod(next);
    setError("");
    setNewAccountId("");
  }, []);

  const handleAuth = useCallback(
    async (e: FormEvent) => {
      e.preventDefault();
      setError("");
      setLoading(true);
      try {
        if (method === "accountId") {
          if (mode === "signin") {
            const u = await doSignInAccountId(accountId);
            setUser(u);
            setAccountId("");
          } else {
            const u = await doRegisterAnonymous();
            setNewAccountId(u.account_id);
            setUser(u);
            downloadAccountFile();
          }
        } else {
          const fn = mode === "signin" ? doSignIn : doSignUp;
          const u = await fn(email, password);
          setUser(u);
          setEmail("");
          setPassword("");
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Auth failed");
      } finally {
        setLoading(false);
      }
    },
    [email, password, accountId, mode, method]
  );

  const connectSocksFallback = useCallback(async (reason: string) => {
    const result = await invoke<ConnectResult>("connect_socks", {
      config: {
        host: DEFAULT_PROXY.host,
        port: DEFAULT_PROXY.port,
      },
    });
    if (result.success) {
      setConnected(true);
      setTunnelMode("socks");
      setStatusMsg(`${reason} Using browser-style SOCKS fallback.`);
    } else {
      setStatusMsg(result.message || reason);
    }
  }, []);

  const handleConnect = useCallback(async () => {
    setStatusMsg("");
    await refreshSession();
    const token = getStoredToken();
    if (!token) {
      setStatusMsg("Not signed in");
      return;
    }

    try {
      const available = await invoke<boolean>("wireguard_available");
      if (!available) {
        await connectSocksFallback("VPN engine missing from app bundle.");
        return;
      }

      const keys = await invoke<KeyPair>("generate_wg_keys");

      const doCreatePeer = async (): Promise<PeerResponse> => {
        const res = await fetch(`${AUTH_API}/api/v1/wg/peers`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ public_key: keys.public_key }),
          maxRedirections: 0,
        });
        const data = (await res.json()) as PeerResponse & { code?: string };
        if (!res.ok) throw data;
        return data;
      };

      let peer: PeerResponse & { code?: string };
      try {
        peer = await doCreatePeer();
      } catch (firstErr) {
        const errData = firstErr as PeerResponse & { code?: string };
        if (!errData.code?.startsWith("plan_device_limit")) {
          throw new Error(errData.error || "Failed to create WireGuard peer");
        }
        // Try to clean up a leftover peer from a previous
        // session (e.g. the user killed the app manually).
        const savedPid = await invoke<string>("saved_peer_id");
        if (savedPid) {
          await fetch(`${AUTH_API}/api/v1/wg/peers/${savedPid}`, {
            method: "DELETE",
            headers: { Authorization: `Bearer ${token}` },
            maxRedirections: 0,
          }).catch(() => undefined);
          await invoke("clear_saved_state");
          try {
            peer = await doCreatePeer();
          } catch (retryErr) {
            const retryData = retryErr as PeerResponse & { code?: string };
            throw new Error(
              retryData.error ||
                "Device limit reached. Upgrade to Premium for more devices, or disconnect another device first."
            );
          }
        } else {
          throw new Error(
            errData.error ||
              "Device limit reached. Upgrade to Premium for more devices, or disconnect another device first."
          );
        }
      }

      const allowed =
        peer.client_allowed_ips ||
        peer.allowed_ips ||
        ["0.0.0.0/0", "::/0"];

      const result = await invoke<ConnectResult>("connect_wireguard", {
        config: {
          private_key: keys.private_key,
          address: peer.assigned_ip,
          dns: peer.dns_server || "1.1.1.1",
          server_public_key: peer.server_public_key,
          endpoint: peer.server_endpoint,
          allowed_ips: allowed,
          peer_id: peer.peer_id,
          preshared_key: peer.preshared_key || "",
        },
      });

      if (result.success) {
        setConnected(true);
        setTunnelMode("wireguard");
        setPeerId(peer.peer_id);
        setStatusMsg("Connected via WireGuard");
      } else {
        setStatusMsg(result.message);
        // Tunnel setup failed — clean up the orphaned peer on the server
        // so it doesn't count against the device limit on retry.
        await fetch(`${AUTH_API}/api/v1/wg/peers/${peer.peer_id}`, {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
          maxRedirections: 0,
        }).catch(() => undefined);
      }
    } catch (err) {
      setStatusMsg(
        err instanceof Error ? err.message : "Connection failed"
      );
    }
  }, [connectSocksFallback]);

  const handleDisconnect = useCallback(async () => {
    setDisconnecting(true);
    setStatusMsg("Disconnecting…");

    // 1. Tear down local tunnel first — this MUST happen even when
    //    the internet is broken because of a malfunctioning tunnel.
    let localOk = false;
    try {
      if (tunnelMode === "wireguard" || peerId) {
        const result = await invoke<ConnectResult>("disconnect_wireguard");
        localOk = result.success;
        if (!localOk) {
          setStatusMsg(
            result.message ||
              "Disconnect incomplete — approve the admin prompt, or run: sudo bash ~/Library/Application\\ Support/cloud.veritasvpn.desktop/teardown.sh"
          );
          setDisconnecting(false);
          return;
        }
      }
      if (tunnelMode === "socks") {
        const result = await invoke<ConnectResult>("disconnect_socks");
        localOk = result.success;
      }
    } catch (err) {
      setStatusMsg(
        err instanceof Error
          ? err.message
          : "Disconnect failed — approve the macOS admin prompt"
      );
      setDisconnecting(false);
      return;
    }

    // Only clear UI after local teardown succeeds.
    setConnected(false);
    setTunnelMode("");
    setPeerId("");
    setDisconnecting(false);
    setStatusMsg("Disconnected");

    // 2. Server cleanup — fire and forget, best-effort.
    if (tunnelMode === "wireguard" && peerId) {
      const token = getStoredToken();
      if (token) {
        const del = fetch(`${AUTH_API}/api/v1/wg/peers/${peerId}`, {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
          maxRedirections: 0,
        });
        const timeout = new Promise<void>((res) => setTimeout(res, 5000));
        Promise.race([del, timeout]).catch(() => undefined);
      }
    }
  }, [tunnelMode, peerId]);

  const handleSignOut = useCallback(async () => {
    if (connected) {
      await handleDisconnect();
    } else {
      // The tunnel may have been killed externally (manual process kill)
      // but the peer still lives on the server. Clean it up now
      // so the next sign-in doesn't hit "device limit reached".
      const pid = await invoke<string>("saved_peer_id");
      if (pid) {
        const token = getStoredToken();
        if (token) {
          const del = fetch(`${AUTH_API}/api/v1/wg/peers/${pid}`, {
            method: "DELETE",
            headers: { Authorization: `Bearer ${token}` },
            maxRedirections: 0,
          });
          const timeout = new Promise<void>((res) => setTimeout(res, 5000));
          await Promise.race([del, timeout]).catch(() => undefined);
          invoke("clear_saved_state");
        }
      }
    }
    doSignOut();
    setUser(null);
    setNewAccountId("");
  }, [connected, handleDisconnect]);

  if (!user || (newAccountId && method === "accountId" && mode === "signup")) {
    const showingNewId = Boolean(newAccountId);
    return (
      <div className="app">
        <div className="brand">
          <h1>Veritas<span>VPN</span></h1>
          <p>Privacy is truth.</p>
        </div>
        {!showingNewId && (
          <div className="auth-tabs">
            <button
              className={mode === "signin" ? "active" : ""}
              onClick={() => switchMode("signin")}
              type="button"
            >
              Sign in
            </button>
            <button
              className={mode === "signup" ? "active" : ""}
              onClick={() => switchMode("signup")}
              type="button"
            >
              Sign up
            </button>
          </div>
        )}
        <form onSubmit={handleAuth}>
          {error && <div className="error">{error}</div>}
          {showingNewId ? (
            <>
              <p className="auth-hint success">
                Your Account ID (copy it now — it cannot be recovered):
              </p>
              <code className="account-id-display">{newAccountId}</code>
              <button
                type="button"
                className="btn-primary"
                onClick={() => {
                  setNewAccountId("");
                }}
              >
                Continue
              </button>
            </>
          ) : method === "email" ? (
            <>
              <input
                type="email"
                placeholder="Email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="email"
              />
              <input
                type="password"
                placeholder="Password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={6}
                autoComplete={mode === "signin" ? "current-password" : "new-password"}
              />
              <button type="submit" disabled={loading} className="btn-primary">
                {loading
                  ? "Please wait..."
                  : mode === "signin"
                    ? "Sign in"
                    : "Create account"}
              </button>
            </>
          ) : mode === "signin" ? (
            <>
              <input
                type="text"
                placeholder="Account ID"
                value={accountId}
                onChange={(e) => setAccountId(e.target.value)}
                required
                autoComplete="off"
                spellCheck={false}
              />
              <button type="submit" disabled={loading} className="btn-primary">
                {loading ? "Please wait..." : "Sign in with Account ID"}
              </button>
            </>
          ) : (
            <>
              <p className="auth-hint">
                Creates an anonymous account. You’ll get an Account ID to save —
                no email required.
              </p>
              <button type="submit" disabled={loading} className="btn-primary">
                {loading ? "Please wait..." : "Create anonymous account"}
              </button>
            </>
          )}
        </form>
        {!showingNewId && (
          <button
            type="button"
            className="auth-switch-link"
            onClick={() =>
              switchMethod(method === "email" ? "accountId" : "email")
            }
          >
            {method === "email"
              ? mode === "signin"
                ? "Sign in with Account ID instead"
                : "Skip email — create anonymous account"
              : mode === "signin"
                ? "Sign in with email instead"
                : "Use email instead"}
          </button>
        )}
      </div>
    );
  }

  return (
    <div className="app">
      <div className="brand">
        <h1>Veritas<span>VPN</span></h1>
        <p className="user-email">
          {user.email || user.account_id}
        </p>
        {user.email && user.account_id && (
          <p className="user-account-id">ID: {user.account_id}</p>
        )}
      </div>
      <div className="status-badge">
        <span className={`dot ${connected ? "on" : "off"}`} />
        {connected
          ? tunnelMode === "wireguard"
            ? "Connected — WireGuard tunnel"
            : "Connected — SOCKS fallback"
          : "Disconnected"}
      </div>
      {statusMsg && <div className="status-msg">{statusMsg}</div>}
      <div className="network-map">
        <svg viewBox="0 0 600 300" xmlns="http://www.w3.org/2000/svg">
          <circle cx="300" cy="150" r="145" fill="none" stroke="var(--border)" strokeWidth="0.5" />
          <text x="300" y="28" textAnchor="middle" fill="var(--text-muted)" fontSize="11">Network map</text>
          <g transform="translate(300,150)">
            <circle r="60" fill="var(--surface)" opacity="0.3" />
            <circle r="3" fill="var(--accent)">
              <animate attributeName="r" values="3;6;3" dur="2s" repeatCount="indefinite" />
              <animate attributeName="opacity" values="1;0.3;1" dur="2s" repeatCount="indefinite" />
            </circle>
            <text y="22" textAnchor="middle" fill="var(--text)" fontSize="12" fontWeight="600">Paraguay</text>
            <text y="36" textAnchor="middle" fill="var(--text-muted)" fontSize="11">Asunción metro</text>
          </g>
        </svg>
      </div>
      {!connected ? (
        <button className="btn-connect" onClick={handleConnect}>
          Connect
        </button>
      ) : (
        <button className="btn-disconnect" onClick={handleDisconnect} disabled={disconnecting}>
          {disconnecting ? "Disconnecting…" : "Disconnect"}
        </button>
      )}
      <button className="btn-signout" onClick={handleSignOut}>
        Sign out
      </button>
      <p className="footer-note">
        One-click WireGuard VPN — no extra software to install
      </p>
    </div>
  );
}

export default App;
