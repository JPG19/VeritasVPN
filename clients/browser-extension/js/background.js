import { getSession, disconnect } from './auth.js';

chrome.runtime.onInstalled.addListener(async () => {
  const session = await getSession();
  if (session.connected) {
    await chrome.action.setBadgeText({ text: 'ON' });
    await chrome.action.setBadgeBackgroundColor({ color: '#00A8FF' });
  }
});

chrome.runtime.onStartup.addListener(async () => {
  const session = await getSession();
  if (!session.connected) {
    await chrome.action.setBadgeText({ text: '' });
  }
});

// If the user clears site data / signs out elsewhere, keep proxy sane on browser restart.
chrome.alarms.create('veritas-health', { periodInMinutes: 30 });
chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name !== 'veritas-health') return;
  const session = await getSession();
  if (!session.user && session.connected) {
    await disconnect();
  }
});
