# VeritasVPN Chrome extension

## Useful information (humans)

Manifest V3 browser extension that lets users:

1. Sign in with the same Firebase account as the website
2. Connect / disconnect from the popup
3. Optionally route Chrome traffic through a SOCKS/HTTP proxy (`chrome.proxy`)

**Install (developer / local):**

1. Open `chrome://extensions`
2. Enable **Developer mode**
3. **Load unpacked** → select this folder (`clients/browser-extension`)
4. Or download the ZIP from the website Downloads page and load the unzipped folder

**Chrome Web Store:** create a developer account, zip this directory (without `.git`), upload, then replace the website Chrome button with the store URL.

**Important:** Without a proxy host configured, Connect runs in **demo mode** (UI + badge only). A real Veritas SOCKS/HTTP proxy must be deployed before browser traffic is protected.

## Useful information (AI)

- Entry: `manifest.json`, popup `popup.html` + `js/popup.js`, SW `js/background.js`
- Auth via Identity Toolkit REST (no bundler); config in `js/config.js`
- Proxy settings in `chrome.storage.local` key `veritas_proxy`
- Do not claim WireGuard inside Chrome — this is browser-proxy protection only
- Website install guide: `website/install/chrome.html`
- Zip artifact for downloads: `website/downloads/veritasvpn-chrome.zip` (regenerate after changes)
