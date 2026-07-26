import { FIREBASE_API_KEY } from "./config";

interface FirebaseAuthResponse {
  idToken: string;
  email: string;
  refreshToken: string;
  expiresIn: string;
  localId: string;
  registered?: boolean;
}

interface FirebaseError {
  error: {
    code: number;
    message: string;
    errors?: Array<{ message: string }>;
  };
}

const STORAGE_KEYS = {
  user: "veritas_user",
  idToken: "veritas_id_token",
  refreshToken: "veritas_refresh_token",
};

export interface User {
  email: string;
  localId: string;
}

function humanizeError(code: string): string {
  switch (code) {
    case "EMAIL_NOT_FOUND":
    case "INVALID_PASSWORD":
    case "INVALID_LOGIN_CREDENTIALS":
      return "Incorrect email or password.";
    case "USER_DISABLED":
      return "This account has been disabled.";
    case "EMAIL_EXISTS":
      return "An account with this email already exists.";
    case "TOO_MANY_ATTEMPTS_TRY_LATER":
      return "Too many attempts. Try again later.";
    default:
      return code.replace(/_/g, " ").toLowerCase();
  }
}

async function firebaseAuth(
  endpoint: string,
  body: Record<string, unknown>
): Promise<FirebaseAuthResponse> {
  const url = `https://identitytoolkit.googleapis.com/v1/accounts:${endpoint}?key=${FIREBASE_API_KEY}`;
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    const fbError = data as FirebaseError;
    const msg =
      fbError?.error?.message || "Authentication failed";
    throw new Error(humanizeError(msg));
  }
  return data as FirebaseAuthResponse;
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
  return localStorage.getItem(STORAGE_KEYS.idToken);
}

export async function signIn(
  email: string,
  password: string
): Promise<User> {
  const data = await firebaseAuth("signInWithPassword", {
    email,
    password,
    returnSecureToken: true,
  });
  const user: User = { email: data.email, localId: data.localId };
  localStorage.setItem(STORAGE_KEYS.user, JSON.stringify(user));
  localStorage.setItem(STORAGE_KEYS.idToken, data.idToken);
  localStorage.setItem(STORAGE_KEYS.refreshToken, data.refreshToken);
  return user;
}

export async function signUp(
  email: string,
  password: string
): Promise<User> {
  const data = await firebaseAuth("signUp", {
    email,
    password,
    returnSecureToken: true,
  });
  const user: User = { email: data.email, localId: data.localId };
  localStorage.setItem(STORAGE_KEYS.user, JSON.stringify(user));
  localStorage.setItem(STORAGE_KEYS.idToken, data.idToken);
  localStorage.setItem(STORAGE_KEYS.refreshToken, data.refreshToken);
  return user;
}

export function signOut(): void {
  localStorage.removeItem(STORAGE_KEYS.user);
  localStorage.removeItem(STORAGE_KEYS.idToken);
  localStorage.removeItem(STORAGE_KEYS.refreshToken);
}
