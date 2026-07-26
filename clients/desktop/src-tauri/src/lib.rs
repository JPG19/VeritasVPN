use serde::{Deserialize, Serialize};
use std::process::Command;

#[derive(Debug, Serialize, Deserialize)]
pub struct ProxyConfig {
    pub host: String,
    pub port: u16,
}

#[derive(Debug, Serialize)]
pub struct ConnectResult {
    pub success: bool,
    pub message: String,
    pub proxy_host: String,
    pub proxy_port: u16,
}

#[tauri::command]
fn connect(config: ProxyConfig) -> ConnectResult {
    match set_system_proxy(&config.host, config.port) {
        Ok(msg) => ConnectResult {
            success: true,
            message: msg,
            proxy_host: config.host.clone(),
            proxy_port: config.port,
        },
        Err(e) => ConnectResult {
            success: false,
            message: e,
            proxy_host: config.host,
            proxy_port: config.port,
        },
    }
}

#[tauri::command]
fn disconnect() -> ConnectResult {
    match remove_system_proxy() {
        Ok(msg) => ConnectResult {
            success: true,
            message: msg,
            proxy_host: String::new(),
            proxy_port: 0,
        },
        Err(e) => ConnectResult {
            success: false,
            message: e,
            proxy_host: String::new(),
            proxy_port: 0,
        },
    }
}

fn set_system_proxy(host: &str, port: u16) -> Result<String, String> {
    #[cfg(target_os = "macos")]
    {
        return set_proxy_macos(host, port);
    }
    #[cfg(target_os = "windows")]
    {
        return set_proxy_windows(host, port);
    }
    #[cfg(target_os = "linux")]
    {
        return set_proxy_linux(host, port);
    }
    #[allow(unreachable_code)]
    Err("Unsupported platform".into())
}

fn remove_system_proxy() -> Result<String, String> {
    #[cfg(target_os = "macos")]
    {
        return remove_proxy_macos();
    }
    #[cfg(target_os = "windows")]
    {
        return remove_proxy_windows();
    }
    #[cfg(target_os = "linux")]
    {
        return remove_proxy_linux();
    }
    #[allow(unreachable_code)]
    Err("Unsupported platform".into())
}

#[cfg(target_os = "macos")]
fn get_active_network_service() -> Result<String, String> {
    let output = Command::new("sh")
        .arg("-c")
        .arg("networksetup -listnetworkserviceorder | grep -B1 \"$(route -n get default 2>/dev/null | grep interface | awk '{print $2}')\" | head -1 | sed 's/^([0-9]*) //'")
        .output()
        .map_err(|e| format!("Failed to detect network service: {}", e))?;

    let service = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if service.is_empty() {
        // Fallback: try Wi-Fi then Ethernet
        for candidate in &["Wi-Fi", "Ethernet", "USB 10/100/1000 LAN"] {
            let check = Command::new("networksetup")
                .args(["-getinfo", candidate])
                .output();
            if check.is_ok() {
                return Ok(candidate.to_string());
            }
        }
        return Err("Could not detect active network service".into());
    }
    Ok(service)
}

#[cfg(target_os = "macos")]
fn set_proxy_macos(host: &str, port: u16) -> Result<String, String> {
    let service = get_active_network_service()?;
    let port_str = port.to_string();

    Command::new("networksetup")
        .args(["-setsocksfirewallproxy", &service, host, &port_str])
        .output()
        .map_err(|e| format!("Failed to set SOCKS proxy: {}", e))?;

    Command::new("networksetup")
        .args(["-setsocksfirewallproxystate", &service, "on"])
        .output()
        .map_err(|e| format!("Failed to enable SOCKS proxy: {}", e))?;

    Ok(format!("SOCKS5 proxy set on {} -> {}:{}", service, host, port))
}

#[cfg(target_os = "macos")]
fn remove_proxy_macos() -> Result<String, String> {
    let service = get_active_network_service()?;

    Command::new("networksetup")
        .args(["-setsocksfirewallproxystate", &service, "off"])
        .output()
        .map_err(|e| format!("Failed to disable SOCKS proxy: {}", e))?;

    Ok(format!("SOCKS5 proxy disabled on {}", service))
}

#[cfg(target_os = "windows")]
fn set_proxy_windows(host: &str, port: u16) -> Result<String, String> {
    use winreg::enums::*;
    use winreg::RegKey;

    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let proxy_path = "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings";
    let settings = hkcu
        .open_subkey_with_flags(proxy_path, KEY_SET_VALUE)
        .map_err(|e| format!("Failed to open registry: {}", e))?;

    let proxy_addr = format!("{}:{}", host, port);
    settings
        .set_value("ProxyServer", &proxy_addr)
        .map_err(|e| format!("Failed to set ProxyServer: {}", e))?;
    settings
        .set_value("ProxyEnable", &1u32)
        .map_err(|e| format!("Failed to enable proxy: {}", e))?;

    Ok(format!("SOCKS5 proxy set -> {}:{}", host, port))
}

#[cfg(target_os = "windows")]
fn remove_proxy_windows() -> Result<String, String> {
    use winreg::enums::*;
    use winreg::RegKey;

    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let proxy_path = "Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings";
    let settings = hkcu
        .open_subkey_with_flags(proxy_path, KEY_SET_VALUE)
        .map_err(|e| format!("Failed to open registry: {}", e))?;
    settings
        .set_value("ProxyEnable", &0u32)
        .map_err(|e| format!("Failed to disable proxy: {}", e))?;

    Ok("System proxy disabled".into())
}

#[cfg(target_os = "linux")]
fn set_proxy_linux(host: &str, port: u16) -> Result<String, String> {
    let port_str = port.to_string();

    let _ = Command::new("gsettings")
        .args(["set", "org.gnome.system.proxy", "mode", "'manual'"])
        .output();
    let _ = Command::new("gsettings")
        .args([
            "set",
            "org.gnome.system.proxy.socks",
            "host",
            &format!("'{}'", host),
        ])
        .output();
    let _ = Command::new("gsettings")
        .args([
            "set",
            "org.gnome.system.proxy.socks",
            "port",
            &port_str,
        ])
        .output();

    Ok(format!("SOCKS5 proxy set (GNOME) -> {}:{}", host, port))
}

#[cfg(target_os = "linux")]
fn remove_proxy_linux() -> Result<String, String> {
    let _ = Command::new("gsettings")
        .args(["set", "org.gnome.system.proxy", "mode", "'none'"])
        .output();

    Ok("System proxy disabled (GNOME)".into())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![connect, disconnect])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
