import { AUTH_API } from "./config";

interface AuthResponse {
  access_token: string;
  refresh_token: string;
  account_id: string;
  expires_at: number;
  email?: string;
}

interface AuthError {
  error: string;
}

const STORAGE_KEYS = {
  user: "veritas_user",
  accessToken: "veritas_access_token",
  refreshToken: "veritas_refresh_token",
};

export interface User {
  email: string;
  account_id: string;
}

function humanizeError(msg: string): string {
  const m = msg.toLowerCase();
  if (m.includes("email")) return "Invalid email address.";
  if (m.includes("password")) {
    if (m.includes("6")) return "Password must be at least 6 characters.";
    return "Incorrect email or password.";
  }
  if (m.includes("already exists")) return "An account with this email already exists.";
  return msg;
}

async function authAPI(
  path: string,
  body: Record<string, unknown>
): Promise<AuthResponse> {
  const url = `${AUTH_API}${path}`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    const err = data as AuthError;
    throw new Error(humanizeError(err?.error || "Authentication failed"));
  }
  return data as AuthResponse;
}

export function getStoredUser(): User | null {
  const raw = localStorage.getItem(STORAGE_KEYS.user);
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

export function getStoredToken(): string | null {
  return localStorage.getItem(STORAGE_KEYS.accessToken);
}

export async function signIn(
  email: string,
  password: string
): Promise<User> {
  const data = await authAPI("/api/v1/auth/signin", {
    email,
    password,
  });
  const user: User = { email: data.email || email, account_id: data.account_id };
  localStorage.setItem(STORAGE_KEYS.user, JSON.stringify(user));
  localStorage.setItem(STORAGE_KEYS.accessToken, data.access_token);
  localStorage.setItem(STORAGE_KEYS.refreshToken, data.refresh_token);
  return user;
}

export async function signUp(
  email: string,
  password: string
): Promise<User> {
  const data = await authAPI("/api/v1/auth/register", {
    email,
    password,
  });
  const user: User = { email: email, account_id: data.account_id };
  localStorage.setItem(STORAGE_KEYS.user, JSON.stringify(user));
  localStorage.setItem(STORAGE_KEYS.accessToken, data.access_token);
  localStorage.setItem(STORAGE_KEYS.refreshToken, data.refresh_token);
  return user;
}

export function signOut(): void {
  localStorage.removeItem(STORAGE_KEYS.user);
  localStorage.removeItem(STORAGE_KEYS.accessToken);
  localStorage.removeItem(STORAGE_KEYS.refreshToken);
}
