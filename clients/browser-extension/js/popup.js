import {
  getSession,
  signIn,
  signUp,
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

function setMode(next) {
  mode = next;
  document.querySelectorAll('.tab').forEach((tab) => {
    tab.classList.toggle('is-active', tab.dataset.mode === mode);
  });
  authSubmit.textContent = mode === 'signin' ? 'Sign in' : 'Create account';
}

document.querySelectorAll('.tab').forEach((tab) => {
  tab.addEventListener('click', () => setMode(tab.dataset.mode));
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
    return;
  }

  authView.hidden = true;
  mainView.hidden = false;
  userEmail.textContent = session.user.email || session.user.localId;
  proxyScheme.value = session.proxy.scheme || 'socks5';
  proxyHost.value = session.proxy.host || '';
  proxyPort.value = session.proxy.port || 1080;
  renderConnected(session.connected, session.connected && !proxyConfigured(session.proxy));
}

authForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  showBanner('');
  const email = document.getElementById('email').value.trim();
  const password = document.getElementById('password').value;
  authSubmit.disabled = true;
  try {
    if (mode === 'signin') {
      await signIn(email, password);
    } else {
      await signUp(email, password);
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
  showBanner('');
  await refresh();
});

saveProxyBtn.addEventListener('click', async () => {
  await saveProxy({
    scheme: proxyScheme.value,
    host: proxyHost.value.trim(),
    port: Number(proxyPort.value) || 1080,
  });
  showBanner('Proxy settings saved.');
  const session = await getSession();
  if (session.connected) {
    await disconnect();
    const result = await connect();
    renderConnected(true, result.demo);
  }
});

refresh();
