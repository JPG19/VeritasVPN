import {
  getSession,
  signIn,
  signUp,
  signInWithAccountId,
  registerAnonymous,
  signOut,
  connect,
  disconnect,
  saveProxy,
  proxyConfigured,
} from './auth.js';

const authView = document.getElementById('authView');
const mainView = document.getElementById('mainView');
const authForm = document.getElementById('authForm');
const authSubmit = document.getElementById('authSubmit');
const authMethodToggle = document.getElementById('authMethodToggle');
const emailFields = document.getElementById('emailFields');
const accountIdFields = document.getElementById('accountIdFields');
const anonHint = document.getElementById('anonHint');
const emailInput = document.getElementById('email');
const passwordInput = document.getElementById('password');
const accountIdInput = document.getElementById('accountId');
const statusBanner = document.getElementById('statusBanner');
const userEmail = document.getElementById('userEmail');
const toggleBtn = document.getElementById('toggleBtn');
const toggleLabel = document.getElementById('toggleLabel');
const connectionState = document.getElementById('connectionState');
const signOutBtn = document.getElementById('signOutBtn');
const proxyScheme = document.getElementById('proxyScheme');
const proxyHost = document.getElementById('proxyHost');
const proxyPort = document.getElementById('proxyPort');
const saveProxyBtn = document.getElementById('saveProxyBtn');

let mode = 'signin';
let method = 'email'; // 'email' | 'accountId'

function showBanner(message, isError = false) {
  if (!message) {
    statusBanner.hidden = true;
    statusBanner.textContent = '';
    return;
  }
  statusBanner.hidden = false;
  statusBanner.textContent = message;
  statusBanner.classList.toggle('error', isError);
}

function syncAuthFields() {
  const useEmail = method === 'email';
  emailFields.hidden = !useEmail;
  accountIdFields.hidden = !(method === 'accountId' && mode === 'signin');
  anonHint.hidden = !(method === 'accountId' && mode === 'signup');

  emailInput.required = useEmail;
  passwordInput.required = useEmail;
  accountIdInput.required = method === 'accountId' && mode === 'signin';

  if (method === 'email') {
    authSubmit.textContent = mode === 'signin' ? 'Sign in' : 'Create account';
    authMethodToggle.textContent =
      mode === 'signin'
        ? 'Sign in with Account ID instead'
        : 'Skip email — create anonymous account';
  } else if (mode === 'signin') {
    authSubmit.textContent = 'Sign in with Account ID';
    authMethodToggle.textContent = 'Sign in with email instead';
  } else {
    authSubmit.textContent = 'Create anonymous account';
    authMethodToggle.textContent = 'Use email instead';
  }
}

function setMode(next) {
  mode = next;
  method = 'email';
  document.querySelectorAll('.tab').forEach((tab) => {
    tab.classList.toggle('is-active', tab.dataset.mode === mode);
  });
  syncAuthFields();
}

document.querySelectorAll('.tab').forEach((tab) => {
  tab.addEventListener('click', () => setMode(tab.dataset.mode));
});

authMethodToggle.addEventListener('click', () => {
  method = method === 'email' ? 'accountId' : 'email';
  showBanner('');
  syncAuthFields();
});

function renderConnected(connected, demo) {
  toggleBtn.setAttribute('aria-pressed', connected ? 'true' : 'false');
  toggleLabel.textContent = connected ? 'Disconnect' : 'Connect';
  if (!connected) {
    connectionState.textContent = 'Disconnected';
  } else if (demo) {
    connectionState.textContent = 'Connected (demo — no proxy host set)';
  } else {
    connectionState.textContent = 'Connected — browser traffic proxied';
  }
}

async function refresh() {
  const session = await getSession();
  if (!session.user) {
    authView.hidden = false;
    mainView.hidden = true;
    renderConnected(false, false);
    syncAuthFields();
    return;
  }

  authView.hidden = true;
  mainView.hidden = false;
  userEmail.textContent = session.user.email || session.user.account_id;
  proxyScheme.value = session.proxy.scheme || 'socks5';
  proxyHost.value = session.proxy.host || '';
  proxyPort.value = session.proxy.port || 1080;
  renderConnected(session.connected, session.connected && !proxyConfigured(session.proxy));
}

authForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  showBanner('');
  authSubmit.disabled = true;
  try {
    if (method === 'accountId') {
      if (mode === 'signin') {
        await signInWithAccountId(accountIdInput.value);
      } else {
        const user = await registerAnonymous();
        showBanner(`Account ID: ${user.account_id} — copy it now.`, false);
      }
    } else if (mode === 'signin') {
      await signIn(emailInput.value.trim(), passwordInput.value);
    } else {
      await signUp(emailInput.value.trim(), passwordInput.value);
    }
    await refresh();
  } catch (err) {
    showBanner(err.message || 'Auth failed', true);
  } finally {
    authSubmit.disabled = false;
  }
});

toggleBtn.addEventListener('click', async () => {
  showBanner('');
  const session = await getSession();
  try {
    if (session.connected) {
      await disconnect();
      renderConnected(false, false);
    } else {
      const result = await connect();
      renderConnected(true, result.demo);
      if (result.demo) {
        showBanner('Demo mode: set a proxy host under Proxy settings for real routing.');
      }
    }
  } catch (err) {
    showBanner(err.message || 'Connection failed', true);
  }
});

signOutBtn.addEventListener('click', async () => {
  await signOut();
  await refresh();
});

saveProxyBtn.addEventListener('click', async () => {
  await saveProxy({
    scheme: proxyScheme.value,
    host: proxyHost.value.trim(),
    port: Number(proxyPort.value) || 1080,
  });
  showBanner('Proxy settings saved.');
});

syncAuthFields();
refresh();
