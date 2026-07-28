import {
  AUTH_API,
  DEFAULT_PROXY,
} from './config.js';

const STORAGE_KEYS = {
  user: 'veritas_user',
  accessToken: 'veritas_access_token',
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
    STORAGE_KEYS.accessToken,
    STORAGE_KEYS.connected,
    STORAGE_KEYS.proxy,
  ]);
  return {
    user: data[STORAGE_KEYS.user] || null,
    idToken: data[STORAGE_KEYS.accessToken] || null,
    connected: Boolean(data[STORAGE_KEYS.connected]),
    proxy: data[STORAGE_KEYS.proxy] || { ...DEFAULT_PROXY },
  };
}

export async function clearSession() {
  await chrome.storage.local.remove([
    STORAGE_KEYS.user,
    STORAGE_KEYS.accessToken,
    STORAGE_KEYS.refreshToken,
    STORAGE_KEYS.connected,
  ]);
  await clearProxy();
}

async function authAPI(endpoint, body) {
  const url = `${AUTH_API}${endpoint}`;
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    const msg = data?.error || 'Authentication failed';
    throw new Error(humanizeError(msg));
  }
  return data;
}

function humanizeError(msg) {
  const m = msg.toLowerCase();
  if (m.includes('email')) return 'Invalid email address.';
  if (m.includes('password')) {
    if (m.includes('6')) return 'Password must be at least 6 characters.';
    return 'Incorrect email or password.';
  }
  if (m.includes('already exists')) return 'An account already exists with this email.';
  return msg;
}

export async function signIn(email, password) {
  const data = await authAPI('/api/v1/auth/signin', {
    email,
    password,
  });
  const user = { email: data.email || email, account_id: data.account_id };
  await setStorage({
    [STORAGE_KEYS.user]: user,
    [STORAGE_KEYS.accessToken]: data.access_token,
    [STORAGE_KEYS.refreshToken]: data.refresh_token,
  });
  return user;
}

export async function signUp(email, password) {
  const data = await authAPI('/api/v1/auth/register', {
    email,
    password,
  });
  const user = { email: email, account_id: data.account_id };
  await setStorage({
    [STORAGE_KEYS.user]: user,
    [STORAGE_KEYS.accessToken]: data.access_token,
    [STORAGE_KEYS.refreshToken]: data.refresh_token,
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
        bypassList: ['localhost', '127.0.0.1', '<local>'],
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
