# Bundled WireGuard binaries (desktop)

## Useful information (humans)

The macOS app ships a **userspace WireGuard** binary (`wireguard-go`) so customers only install VeritasVPN — no Homebrew / WireGuard.app required.

Rebuild (developers / CI):

```bash
./clients/desktop/scripts/bundle-wg-macos.sh
```

## Useful information (AI)

- Path: `resources/bin/wireguard-go` (arm64 Mach-O)
- Tauri bundles via `bundle.resources` in `tauri.conf.json`
- Runtime: `lib.rs` starts this binary with admin privileges, configures via UAPI + `ifconfig`/`route`
- Do not tell end users to `brew install wireguard-tools`
