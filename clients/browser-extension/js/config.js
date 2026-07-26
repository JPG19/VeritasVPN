/**
 * Shared config for the VeritasVPN Chrome extension.
 * Keep proxy host empty until a real SOCKS/HTTP proxy is deployed —
 * Connect then runs in UI-only demo mode.
 */
export const FIREBASE_API_KEY = 'AIzaSyA1UrJKJ7WkvZp2s31hSn-SSe5ZL_-RmVo';
export const FIREBASE_AUTH_DOMAIN = 'veritasvpn-37cf6.firebaseapp.com';
export const FIREBASE_PROJECT_ID = 'veritasvpn-37cf6';

export const BILLING_API = '';

/** SOCKS5 proxy on the gateway node. */
export const DEFAULT_PROXY = {
  scheme: 'socks5',
  host: '186.122.244.166',
  port: 1080,
};
