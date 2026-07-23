# VeritasVPN Desktop (macOS first)

## Useful information (humans)

Desktop client target stack: **Tauri 2 + React** (not Electron), per the main implementation plan.

This folder is the starter home for the macOS app. Full WireGuard tunnel integration (privileged helper / Network Extension) is still required before a notarized `.dmg` can ship.

**Website offer today:** Downloads → macOS points users here / to build instructions while the DMG is in progress.

### Local scaffold (when ready to develop)

```bash
# From repo root — requires Rust + Node
npm create tauri-app@latest desktop-ui -- --template react-ts
# then merge into clients/desktop/
```

Or follow Phase 5 in `IMPLEMENTATION_PLAN.md` (Tauri 2 + WireGuard controller).

## Useful information (AI)

- Prefer Tauri over Electron for this product
- macOS production path: DMG + notarization + Network Extension / `wireguard-go` helper
- Interim UX: `website/install/macos.html` + featured CTA on `downloads.html`
- Do not advertise a live macOS binary until a release artifact URL exists
