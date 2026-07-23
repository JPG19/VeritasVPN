import { initializeApp } from 'https://www.gstatic.com/firebasejs/12.16.0/firebase-app.js';
import {
  getAuth,
  onAuthStateChanged,
  signInWithEmailAndPassword,
  createUserWithEmailAndPassword,
  signInWithPopup,
  GoogleAuthProvider,
  signOut,
  sendPasswordResetEmail,
} from 'https://www.gstatic.com/firebasejs/12.16.0/firebase-auth.js';
import { firebaseConfig } from './firebase-config.js';

const app = initializeApp(firebaseConfig);
export const auth = getAuth(app);
const googleProvider = new GoogleAuthProvider();

export const ACCOUNT_PATH = '/account/';

export async function getIdToken() {
  const user = auth.currentUser;
  if (!user) return null;
  return user.getIdToken();
}

export function goToDashboard(hash = '') {
  const path = hash ? `${ACCOUNT_PATH}${hash}` : ACCOUNT_PATH;
  window.location.href = path;
}

export function requireAuthOrOpenModal(preferredMode = 'signin') {
  if (auth.currentUser) return true;
  const btn =
    document.querySelector(`[data-auth-open="${preferredMode}"]`) ||
    document.querySelector('[data-auth-open]');
  btn?.click();
  return false;
}

function mapAuthError(error) {
  const code = error?.code || '';
  switch (code) {
    case 'auth/invalid-email':
      return 'Enter a valid email address.';
    case 'auth/user-disabled':
      return 'This account has been disabled.';
    case 'auth/user-not-found':
    case 'auth/wrong-password':
    case 'auth/invalid-credential':
      return 'Incorrect email or password.';
    case 'auth/email-already-in-use':
      return 'An account already exists with this email.';
    case 'auth/weak-password':
      return 'Password must be at least 6 characters.';
    case 'auth/operation-not-allowed':
      return 'This sign-in method is disabled in Firebase. Enable Email/Password (or Google) under Authentication → Sign-in method.';
    case 'auth/popup-closed-by-user':
      return 'Sign-in popup was closed before completing.';
    case 'auth/popup-blocked':
      return 'Pop-up was blocked. Allow pop-ups for this site and try again.';
    case 'auth/too-many-requests':
      return 'Too many attempts. Please try again later.';
    case 'auth/network-request-failed':
      return 'Network error. Check your connection and try again.';
    default:
      return error?.message || 'Something went wrong. Please try again.';
  }
}

function shouldRedirectToDashboardAfterAuth() {
  // Stay on account app if already there; otherwise enter dashboard after login.
  if (window.location.pathname.startsWith('/account')) return false;
  return true;
}

export function initAuthUI({ redirectAfterAuth = true } = {}) {
  const modal = document.getElementById('authModal');
  const openButtons = document.querySelectorAll('[data-auth-open]');
  const gateButtons = document.querySelectorAll('[data-auth-gate]');
  const closeButtons = document.querySelectorAll('[data-auth-close]');
  const tabs = document.querySelectorAll('[data-auth-tab]');
  const form = document.getElementById('authForm');
  const emailInput = document.getElementById('authEmail');
  const passwordInput = document.getElementById('authPassword');
  const submitBtn = document.getElementById('authSubmit');
  const googleBtn = document.getElementById('authGoogle');
  const resetBtn = document.getElementById('authReset');
  const errorEl = document.getElementById('authError');
  const titleEl = document.getElementById('authTitle');
  const switchHint = document.getElementById('authSwitchHint');
  const loggedOut = document.getElementById('navAuthLoggedOut');
  const loggedIn = document.getElementById('navAuthLoggedIn');
  const userEmailEl = document.getElementById('navUserEmail');
  const userMenuBtn = document.getElementById('navUserMenuBtn');
  const userMenu = document.getElementById('navUserMenu');
  const signOutBtn = document.getElementById('authSignOut');
  const dashboardLinks = document.querySelectorAll('[data-open-dashboard]');

  let mode = 'signin';
  let busy = false;
  let pendingDashboardRedirect = false;
  let authReady = false;

  function setError(message, { success = false } = {}) {
    if (!errorEl) return;
    errorEl.textContent = message || '';
    errorEl.hidden = !message;
    errorEl.classList.toggle('is-success', Boolean(message) && success);
  }

  function setMode(next) {
    mode = next;
    const isSignIn = mode === 'signin';
    tabs.forEach((tab) => {
      const active = tab.dataset.authTab === mode;
      tab.classList.toggle('is-active', active);
      tab.setAttribute('aria-selected', active ? 'true' : 'false');
    });
    if (titleEl) titleEl.textContent = isSignIn ? 'Sign in' : 'Create account';
    if (submitBtn) submitBtn.textContent = isSignIn ? 'Sign in' : 'Create account';
    if (passwordInput) {
      passwordInput.autocomplete = isSignIn ? 'current-password' : 'new-password';
    }
    if (switchHint) {
      switchHint.textContent = isSignIn
        ? "Don't have an account? Sign up"
        : 'Already have an account? Sign in';
    }
    if (resetBtn) resetBtn.hidden = !isSignIn;
    setError('');
  }

  function openModal(preferredMode = 'signin') {
    setMode(preferredMode);
    modal?.classList.add('is-open');
    modal?.setAttribute('aria-hidden', 'false');
    document.body.classList.add('auth-modal-open');
    setTimeout(() => emailInput?.focus(), 50);
  }

  function closeModal() {
    modal?.classList.remove('is-open');
    modal?.setAttribute('aria-hidden', 'true');
    document.body.classList.remove('auth-modal-open');
    setError('');
    form?.reset();
    userMenu?.classList.remove('is-open');
  }

  function setBusy(next) {
    busy = next;
    if (submitBtn) submitBtn.disabled = busy;
    if (googleBtn) googleBtn.disabled = busy;
    if (resetBtn) resetBtn.disabled = busy;
  }

  function enterDashboard(hash = '') {
    goToDashboard(hash);
  }

  function handleGateClick(e, preferredMode = 'signup', hash = '') {
    e.preventDefault();
    if (auth.currentUser) {
      enterDashboard(hash);
      return;
    }
    pendingDashboardRedirect = true;
    openModal(preferredMode);
  }

  function renderUser(user) {
    if (user) {
      loggedOut?.classList.add('is-hidden');
      loggedIn?.classList.remove('is-hidden');
      if (userEmailEl) {
        userEmailEl.textContent = user.email || user.displayName || 'Account';
      }
      closeModal();
      window.dispatchEvent(new CustomEvent('veritas-auth-changed', { detail: { user } }));

      if (
        redirectAfterAuth &&
        pendingDashboardRedirect &&
        shouldRedirectToDashboardAfterAuth()
      ) {
        pendingDashboardRedirect = false;
        enterDashboard();
      }
    } else {
      loggedOut?.classList.remove('is-hidden');
      loggedIn?.classList.add('is-hidden');
      userMenu?.classList.remove('is-open');
      pendingDashboardRedirect = false;
      window.dispatchEvent(new CustomEvent('veritas-auth-changed', { detail: { user: null } }));
    }
  }

  // Also: Log in success from marketing should enter dashboard
  openButtons.forEach((btn) => {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      // Sign-in from marketing should land in dashboard after success
      pendingDashboardRedirect =
        redirectAfterAuth &&
        shouldRedirectToDashboardAfterAuth() &&
        (btn.dataset.authOpen === 'signin' || btn.dataset.authOpen === 'signup');
      openModal(btn.dataset.authOpen || 'signin');
    });
  });

  gateButtons.forEach((btn) => {
    btn.addEventListener('click', (e) => {
      const modeAttr = btn.dataset.authGate || 'signup';
      const hash = btn.dataset.dashboardHash || '';
      handleGateClick(e, modeAttr === 'dashboard' ? 'signup' : modeAttr, hash);
    });
  });

  dashboardLinks.forEach((link) => {
    link.addEventListener('click', (e) => {
      e.preventDefault();
      if (auth.currentUser) {
        enterDashboard(link.dataset.dashboardHash || '');
      } else {
        pendingDashboardRedirect = true;
        openModal('signin');
      }
    });
  });

  closeButtons.forEach((btn) => {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      pendingDashboardRedirect = false;
      closeModal();
    });
  });

  modal?.addEventListener('click', (e) => {
    if (e.target === modal) {
      pendingDashboardRedirect = false;
      closeModal();
    }
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && modal?.classList.contains('is-open')) {
      pendingDashboardRedirect = false;
      closeModal();
    }
  });

  tabs.forEach((tab) => {
    tab.addEventListener('click', () => setMode(tab.dataset.authTab));
  });

  switchHint?.addEventListener('click', (e) => {
    e.preventDefault();
    setMode(mode === 'signin' ? 'signup' : 'signin');
  });

  form?.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (busy) return;
    setError('');
    const email = emailInput?.value.trim() || '';
    const password = passwordInput?.value || '';
    if (!email || !password) {
      setError('Email and password are required.');
      return;
    }
    setBusy(true);
    try {
      pendingDashboardRedirect = redirectAfterAuth && shouldRedirectToDashboardAfterAuth();
      if (mode === 'signin') {
        await signInWithEmailAndPassword(auth, email, password);
      } else {
        await createUserWithEmailAndPassword(auth, email, password);
      }
    } catch (err) {
      pendingDashboardRedirect = false;
      setError(mapAuthError(err));
    } finally {
      setBusy(false);
    }
  });

  googleBtn?.addEventListener('click', async () => {
    if (busy) return;
    setError('');
    setBusy(true);
    try {
      pendingDashboardRedirect = redirectAfterAuth && shouldRedirectToDashboardAfterAuth();
      await signInWithPopup(auth, googleProvider);
    } catch (err) {
      pendingDashboardRedirect = false;
      setError(mapAuthError(err));
    } finally {
      setBusy(false);
    }
  });

  resetBtn?.addEventListener('click', async (e) => {
    e.preventDefault();
    if (busy) return;
    const email = emailInput?.value.trim() || '';
    if (!email) {
      setError('Enter your email above, then click “Forgot password”.');
      return;
    }
    setBusy(true);
    setError('');
    try {
      await sendPasswordResetEmail(auth, email);
      setError('Password reset email sent. Check your inbox.', { success: true });
    } catch (err) {
      setError(mapAuthError(err));
    } finally {
      setBusy(false);
    }
  });

  userMenuBtn?.addEventListener('click', (e) => {
    e.preventDefault();
    userMenu?.classList.toggle('is-open');
  });

  document.addEventListener('click', (e) => {
    if (!loggedIn?.contains(e.target)) {
      userMenu?.classList.remove('is-open');
    }
  });

  signOutBtn?.addEventListener('click', async (e) => {
    e.preventDefault();
    try {
      await signOut(auth);
      if (window.location.pathname.startsWith('/account')) {
        window.location.href = '/';
      }
    } catch (err) {
      console.error(err);
    }
  });

  // Deep link: /?signin=1 or /?signup=1
  const params = new URLSearchParams(window.location.search);
  if (params.get('signin') === '1') {
    openModal('signin');
  } else if (params.get('signup') === '1') {
    openModal('signup');
  }

  onAuthStateChanged(auth, (user) => {
    authReady = true;
    renderUser(user);
  });

  setMode('signin');

  return {
    openModal,
    goToDashboard: enterDashboard,
    isReady: () => authReady,
  };
}

export { signOut, sendPasswordResetEmail, onAuthStateChanged };
