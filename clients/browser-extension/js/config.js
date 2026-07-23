/**
 * Shared config for the VeritasVPN Chrome extension.
 * Keep proxy host empty until a real SOCKS/HTTP proxy is deployed —
 * Connect then runs in UI-only demo mode.
 */
export const FIREBASE_API_KEY = 'AIzaSyA1UrJKJ7WkvZp2s31hSn-SSe5ZL_-RmVo';
export const FIREBASE_AUTH_DOMAIN = 'veritasvpn-37cf6.firebaseapp.com';
export const FIREBASE_PROJECT_ID = 'veritasvpn-37cf6';

export const BILLING_API = 'http://localhost:8083';

/** Default empty = demo mode (no chrome.proxy mutation). */
export const DEFAULT_PROXY = {
  scheme: 'socks5',
  host: '',
  port: 1080,
};
