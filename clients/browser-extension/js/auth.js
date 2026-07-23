import {
  FIREBASE_API_KEY,
  FIREBASE_AUTH_DOMAIN,
  DEFAULT_PROXY,
} from './config.js';

const STORAGE_KEYS = {
  user: 'veritas_user',
  idToken: 'veritas_id_token',
  refreshToken: 'veritas_refresh_token',
  connected: 'veritas_connected',
  proxy: 'veritas_proxy',
};

async function getStorage(keys) {
  return chrome.storage.local.get(keys);
}

async function setStorage(obj) {
  return chrome.storage.local.set(obj);
}

export async function getSession() {
  const data = await getStorage([
    STORAGE_KEYS.user,
    STORAGE_KEYS.idToken,
    STORAGE_KEYS.connected,
    STORAGE_KEYS.proxy,
  ]);
  return {
    user: data[STORAGE_KEYS.user] || null,
    idToken: data[STORAGE_KEYS.idToken] || null,
    connected: Boolean(data[STORAGE_KEYS.connected]),
    proxy: data[STORAGE_KEYS.proxy] || { ...DEFAULT_PROXY },
  };
}

export async function clearSession() {
  await chrome.storage.local.remove([
    STORAGE_KEYS.user,
    STORAGE_KEYS.idToken,
    STORAGE_KEYS.refreshToken,
    STORAGE_KEYS.connected,
  ]);
  await clearProxy();
}

async function firebaseAuth(endpoint, body) {
  const url = `https://identitytoolkit.googleapis.com/v1/accounts:${endpoint}?key=${FIREBASE_API_KEY}`;
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    const msg = data?.error?.message || 'Authentication failed';
    throw new Error(humanizeFirebaseError(msg));
  }
  return data;
}

function humanizeFirebaseError(code) {
  switch (code) {
    case 'EMAIL_NOT_FOUND':
    case 'INVALID_PASSWORD':
    case 'INVALID_LOGIN_CREDENTIALS':
      return 'Incorrect email or password.';
    case 'USER_DISABLED':
      return 'This account has been disabled.';
    case 'TOO_MANY_ATTEMPTS_TRY_LATER':
      return 'Too many attempts. Try again later.';
    default:
      return code.replace(/_/g, ' ').toLowerCase();
  }
}

export async function signIn(email, password) {
  const data = await firebaseAuth('signInWithPassword', {
    email,
    password,
    returnSecureToken: true,
  });
  const user = { email: data.email, localId: data.localId };
  await setStorage({
    [STORAGE_KEYS.user]: user,
    [STORAGE_KEYS.idToken]: data.idToken,
    [STORAGE_KEYS.refreshToken]: data.refreshToken,
  });
  return user;
}

export async function signUp(email, password) {
  const data = await firebaseAuth('signUp', {
    email,
    password,
    returnSecureToken: true,
  });
  const user = { email: data.email, localId: data.localId };
  await setStorage({
    [STORAGE_KEYS.user]: user,
    [STORAGE_KEYS.idToken]: data.idToken,
    [STORAGE_KEYS.refreshToken]: data.refreshToken,
  });
  return user;
}

export async function signOut() {
  await clearSession();
}

function proxyConfigured(proxy) {
  return Boolean(proxy?.host && String(proxy.host).trim());
}

export async function connect() {
  const session = await getSession();
  if (!session.user) {
    throw new Error('Sign in first');
  }

  const proxy = session.proxy;
  if (proxyConfigured(proxy)) {
    const config = {
      mode: 'fixed_servers',
      rules: {
        singleProxy: {
          scheme: proxy.scheme || 'socks5',
          host: proxy.host,
          port: Number(proxy.port) || 1080,
        },
        bypassList: ['localhost', '127.0.0.1', '<local>', FIREBASE_AUTH_DOMAIN],
      },
    };
    await chrome.proxy.settings.set({ value: config, scope: 'regular' });
  }

  await setStorage({ [STORAGE_KEYS.connected]: true });
  await chrome.action.setBadgeText({ text: 'ON' });
  await chrome.action.setBadgeBackgroundColor({ color: '#00A8FF' });
  return { demo: !proxyConfigured(proxy) };
}

export async function disconnect() {
  await clearProxy();
  await setStorage({ [STORAGE_KEYS.connected]: false });
  await chrome.action.setBadgeText({ text: '' });
}

async function clearProxy() {
  try {
    await chrome.proxy.settings.clear({ scope: 'regular' });
  } catch {
    // ignore
  }
}

export async function saveProxy(proxy) {
  await setStorage({ [STORAGE_KEYS.proxy]: proxy });
}

export { STORAGE_KEYS, proxyConfigured };
