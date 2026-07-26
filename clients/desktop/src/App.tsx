import { useState, useEffect, FormEvent, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  getStoredUser,
  signIn as doSignIn,
  signUp as doSignUp,
  signOut as doSignOut,
  type User,
} from "./auth";
import { DEFAULT_PROXY } from "./config";
import "./App.css";

type AuthMode = "signin" | "signup";

interface ConnectResult {
  success: boolean;
  message: string;
}

function App() {
  const [user, setUser] = useState<User | null>(getStoredUser);
  const [mode, setMode] = useState<AuthMode>("signin");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [statusMsg, setStatusMsg] = useState("");

  // Clear status message after 5 seconds
  useEffect(() => {
    if (!statusMsg) return;
    const t = setTimeout(() => setStatusMsg(""), 5000);
    return () => clearTimeout(t);
  }, [statusMsg]);

  const handleAuth = useCallback(
    async (e: FormEvent) => {
      e.preventDefault();
      setError("");
      setLoading(true);
      try {
        const fn = mode === "signin" ? doSignIn : doSignUp;
        const u = await fn(email, password);
        setUser(u);
        setEmail("");
        setPassword("");
      } catch (err) {
        setError(err instanceof Error ? err.message : "Auth failed");
      } finally {
        setLoading(false);
      }
    },
    [email, password, mode]
  );

  const handleConnect = useCallback(async () => {
    setStatusMsg("");
    try {
      const result = await invoke<ConnectResult>("connect", {
        config: {
          host: DEFAULT_PROXY.host,
          port: DEFAULT_PROXY.port,
        },
      });
      if (result.success) {
        setConnected(true);
        setStatusMsg("Connected — system proxy set");
      } else {
        setStatusMsg(result.message);
      }
    } catch (err) {
      setStatusMsg(
        err instanceof Error ? err.message : "Connection failed"
      );
    }
  }, []);

  const handleDisconnect = useCallback(async () => {
    setStatusMsg("");
    try {
      const result = await invoke<ConnectResult>("disconnect");
      if (result.success) {
        setConnected(false);
        setStatusMsg("Disconnected — proxy removed");
      } else {
        setStatusMsg(result.message);
      }
    } catch (err) {
      setStatusMsg(
        err instanceof Error ? err.message : "Disconnect failed"
      );
    }
  }, []);

  const handleSignOut = useCallback(() => {
    if (connected) {
      handleDisconnect();
    }
    doSignOut();
    setUser(null);
  }, [connected, handleDisconnect]);

  if (!user) {
    return (
      <div className="app">
        <div className="brand">
          <h1>Veritas<span>VPN</span></h1>
          <p>Privacy is truth.</p>
        </div>
        <div className="auth-tabs">
          <button
            className={mode === "signin" ? "active" : ""}
            onClick={() => { setMode("signin"); setError(""); }}
          >
            Sign in
          </button>
          <button
            className={mode === "signup" ? "active" : ""}
            onClick={() => { setMode("signup"); setError(""); }}
          >
            Sign up
          </button>
        </div>
        <form onSubmit={handleAuth}>
          {error && <div className="error">{error}</div>}
          <input
            type="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          <input
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={6}
          />
          <button type="submit" disabled={loading} className="btn-primary">
            {loading
              ? "Please wait..."
              : mode === "signin"
                ? "Sign in"
                : "Create account"}
          </button>
        </form>
      </div>
    );
  }

  return (
    <div className="app">
      <div className="brand">
        <h1>Veritas<span>VPN</span></h1>
        <p className="user-email">{user.email}</p>
      </div>
      <div className="status-badge">
        <span className={`dot ${connected ? "on" : "off"}`} />
        {connected ? "Connected — traffic routed through VeritasVPN" : "Disconnected"}
      </div>
      {statusMsg && <div className="status-msg">{statusMsg}</div>}
      {!connected ? (
        <button className="btn-connect" onClick={handleConnect}>
          Connect
        </button>
      ) : (
        <button className="btn-disconnect" onClick={handleDisconnect}>
          Disconnect
        </button>
      )}
      <button className="btn-signout" onClick={handleSignOut}>
        Sign out
      </button>
      <p className="footer-note">
        SOCKS5 via {DEFAULT_PROXY.host}:{DEFAULT_PROXY.port}
      </p>
    </div>
  );
}

export default App;
