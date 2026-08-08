import { useState, useEffect, FormEvent, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import { fetch } from "@tauri-apps/plugin-http";
import {
  getStoredUser,
  getStoredToken,
  refreshSession,
  signIn as doSignIn,
  signUp as doSignUp,
  signInWithAccountId as doSignInAccountId,
  registerAnonymous as doRegisterAnonymous,
  signOut as doSignOut,
  type User,
} from "./auth";
import { AUTH_API } from "./config";
import "./App.css";

type AuthMode = "signin" | "signup";
/** Email/password vs Account ID (anonymous) path. */
type AuthMethod = "email" | "accountId";
type TunnelMode = "wireguard" | "";

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

function ConnectionMap({ connected }: { connected: boolean }) {
  return (
    <section className={`connection-map ${connected ? "is-connected" : ""}`} aria-label="VPN route from your device to Paraguay">
      <div className="map-topline">
        <span>LIVE ROUTE</span>
        <span className="map-latency">{connected ? "Encrypted" : "Ready"}</span>
      </div>
      <svg viewBox="0 0 900 430" role="img" aria-label="World map with a route to the VeritasVPN node in Paraguay">
        <defs>
          <linearGradient id="routeGradient" x1="0" x2="1">
            <stop offset="0" stopColor="#7048ff" />
            <stop offset="1" stopColor="#28d9c3" />
          </linearGradient>
          <filter id="routeGlow">
            <feGaussianBlur stdDeviation="5" result="blur" />
            <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
          </filter>
        </defs>
        <g className="map-grid">
          <path d="M0 108H900M0 215H900M0 322H900M225 0V430M450 0V430M675 0V430" />
        </g>
        <g className="continents">
          <path d="M77 99l50-46 92-13 68 26 37 47-20 43-54 9-22 37-37 11-19 43-35-16-12-61-46-31z" />
          <path d="M248 221l45 27 27 58-17 78-37 37-22-53 8-56-32-53z" />
          <path d="M421 87l47-36 74 6 37 24 72-14 95 29 74 53-24 37-65 3-38 38-42-16-42 22-27-33-55 8-39-38-54-21z" />
          <path d="M487 211l64 8 48 43-16 77-46 53-42-39-17-78z" />
          <path d="M735 311l51-31 66 25 20 52-57 31-65-22z" />
          <path d="M390 405l133-10 116 16-37 19H432z" />
        </g>
        <path className="route-shadow" d="M128 125C255 88 339 164 421 239S535 315 599 319" />
        <path className="route-line" d="M128 125C255 88 339 164 421 239S535 315 599 319" />
        <circle className="route-particle particle-one" r="4">
          <animateMotion dur="2.8s" repeatCount="indefinite" path="M128 125C255 88 339 164 421 239S535 315 599 319" />
        </circle>
        <circle className="route-particle particle-two" r="3">
          <animateMotion begin="1.2s" dur="2.8s" repeatCount="indefinite" path="M128 125C255 88 339 164 421 239S535 315 599 319" />
        </circle>
        <g className="map-origin" transform="translate(128 125)">
          <circle r="5" />
          <circle className="map-pulse" r="12" />
        </g>
        <g className="map-destination" transform="translate(599 319)">
          <circle className="map-pulse" r="18" />
          <circle r="8" />
        </g>
      </svg>
      <div className="route-label route-label-origin">
        <span>Your device</span>
        <strong>Protected locally</strong>
      </div>
      <div className="route-label route-label-destination">
        <span>PARAGUAY</span>
        <strong>Asunción metro</strong>
      </div>
    </section>
  );
}

function App() {
  const [user, setUser] = useState<User | null>(getStoredUser);
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
  const [connecting, setConnecting] = useState(false);

  useEffect(() => {
    if (!statusMsg) return;
    const t = setTimeout(() => setStatusMsg(""), 5000);
    return () => clearTimeout(t);
  }, [statusMsg]);

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

  const handleConnect = useCallback(async () => {
    if (connecting) return;
    setStatusMsg("");
    setConnecting(true);

    let token = "";
    let createdPeerId = "";
    try {
      await refreshSession();
      token = getStoredToken() || "";
      if (!token) {
        throw new Error("Not signed in");
      }

      const available = await invoke<boolean>("wireguard_available");
      if (!available) {
        throw new Error(
          "WireGuard is unavailable in this build. No proxy fallback was activated."
        );
      }

      const keys = await invoke<KeyPair>("generate_wg_keys");
      const res = await fetch(`${AUTH_API}/api/v1/wg/peers`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ public_key: keys.public_key }),
      });
      const peer = (await res.json()) as PeerResponse & { code?: string };
      if (!res.ok) {
        if (peer.code?.startsWith("plan_device_limit")) {
          throw new Error(
            peer.error ||
              "Device limit reached. Upgrade to Premium for more devices, or disconnect another device first."
          );
        }
        throw new Error(peer.error || "Failed to create WireGuard peer");
      }
      createdPeerId = peer.peer_id;

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
        throw new Error(result.message || "WireGuard connection failed");
      }
    } catch (err) {
      // Peer creation precedes privileged local bring-up. Roll it back when
      // bring-up fails so retries do not leave stale keys or allocated IPs.
      if (createdPeerId && token) {
        await fetch(`${AUTH_API}/api/v1/wg/peers/${createdPeerId}`, {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
        }).catch(() => undefined);
      }
      setStatusMsg(err instanceof Error ? err.message : "Connection failed");
    } finally {
      setConnecting(false);
    }
  }, [connecting]);

  const handleDisconnect = useCallback(async () => {
    setStatusMsg("Disconnecting…");
    // Always clear local tunnel UI state so a failed privileged teardown
    // cannot leave the app stuck showing Connected.
    const clearUi = () => {
      setConnected(false);
      setTunnelMode("");
      setPeerId("");
    };
    try {
      if (tunnelMode === "wireguard" || peerId) {
        const token = getStoredToken();
        // Restore local routing and DNS before making any network request.
        // Otherwise a degraded tunnel can block disconnect indefinitely.
        const result = await invoke<ConnectResult>("disconnect_wireguard");
        clearUi();
        if (!result.success) {
          setStatusMsg(
            result.message ||
              "Disconnect incomplete — approve the admin prompt, or run the teardown script from your app config directory"
          );
          return;
        }
        if (token && peerId) {
          const controller = new AbortController();
          const timeout = window.setTimeout(() => controller.abort(), 5000);
          await fetch(`${AUTH_API}/api/v1/wg/peers/${peerId}`, {
            method: "DELETE",
            headers: { Authorization: `Bearer ${token}` },
            signal: controller.signal,
          }).catch(() => undefined).finally(() => window.clearTimeout(timeout));
        }
      }
      clearUi();
      setStatusMsg("Disconnected");
    } catch (err) {
      clearUi();
      setStatusMsg(
        err instanceof Error ? err.message : "Disconnect failed — approve the macOS admin prompt"
      );
    }
  }, [tunnelMode, peerId]);

  const handleSignOut = useCallback(() => {
    if (connected) {
      handleDisconnect();
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
    <div className="app app-dashboard">
      <header className="app-header">
        <div className="brand brand-inline">
          <span className="brand-mark">V</span>
          <div><h1>Veritas<span>VPN</span></h1><p>Privacy is truth.</p></div>
        </div>
        <div className="account-menu">
          <span className="account-avatar">{(user.email || user.account_id || "V").slice(0, 1).toUpperCase()}</span>
          <div>
            <strong>{user.email || "Anonymous account"}</strong>
            <span>Veritas account</span>
          </div>
          <button className="btn-signout" onClick={handleSignOut}>Sign out</button>
        </div>
      </header>

      <main className="dashboard-main">
        <ConnectionMap connected={connected} />
        <aside className="connection-panel">
          <div className={`status-orb ${connected ? "is-on" : ""}`}>
            <span className="orb-ring" />
            <span className="orb-lock">{connected ? "✓" : "V"}</span>
          </div>
          <p className="eyebrow">{connected ? "CONNECTION SECURED" : "VPN READY"}</p>
          <h2>{connected ? "You’re protected" : "Connect to Veritas"}</h2>
          <p className="connection-copy">
            {connected
              ? tunnelMode === "wireguard"
                ? "Your internet traffic is encrypted through our WireGuard node in Paraguay."
                : "Your browser traffic is protected through our encrypted proxy in Paraguay."
              : "Route your traffic privately through our live node in Paraguay."}
          </p>
          <div className="server-card">
            <span className="flag">🇵🇾</span>
            <div><strong>Paraguay</strong><span>Asunción metro · Automatic</span></div>
            <span className="server-live">LIVE</span>
          </div>
          {!connected ? (
            <button
              className="btn-connect"
              onClick={handleConnect}
              disabled={connecting}
            >
              <span>{connecting ? "Connecting…" : "Connect now"}</span>
              <i>→</i>
            </button>
          ) : (
            <button className="btn-disconnect" onClick={handleDisconnect}>Disconnect</button>
          )}
          <div className="connection-meta">
            <span><i className={`dot ${connected ? "on" : "off"}`} />{connected ? "Protected" : "Not connected"}</span>
            <span>{tunnelMode === "wireguard" ? "WireGuard" : "WireGuard only"}</span>
          </div>
          {statusMsg && <div className="status-msg">{statusMsg}</div>}
        </aside>
      </main>
    </div>
  );
}

export default App;
