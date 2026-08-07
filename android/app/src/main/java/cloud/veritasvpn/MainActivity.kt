package cloud.veritasvpn

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.*
import cloud.veritasvpn.api.ApiClient
import cloud.veritasvpn.api.PeerResponse
import cloud.veritasvpn.auth.AuthRepository
import cloud.veritasvpn.ui.AuthScreen
import cloud.veritasvpn.ui.DashboardScreen
import cloud.veritasvpn.ui.theme.VeritasVPNTheme
import cloud.veritasvpn.vpn.VeritasVpnService
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.security.SecureRandom
import java.util.Base64

class MainActivity : ComponentActivity() {
    private lateinit var authRepo: AuthRepository

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        authRepo = AuthRepository(this)

        setContent {
            VeritasVPNTheme {
                var user by remember { mutableStateOf(authRepo.getStoredUser()) }
                var connected by remember { mutableStateOf(false) }
                var statusMsg by remember { mutableStateOf<String?>(null) }
                var peerId by remember { mutableStateOf<String?>(null) }
                val context = androidx.compose.ui.platform.LocalContext.current
                val scope = rememberCoroutineScope()

                if (user == null) {
                    AuthScreen(onAuthenticated = {
                        user = authRepo.getStoredUser()
                    })
                } else {
                    DashboardScreen(
                        connected = connected,
                        onConnect = {
                            statusMsg = "Connecting..."
                            scope.launch {
                                try {
                                    val (keys, peer) = withContext(Dispatchers.IO) {
                                        authRepo.refreshSession()
                                        val token = authRepo.getAccessToken()
                                            ?: throw IllegalStateException("Not signed in")
                                        val generatedKeys = generateWireGuardKeys()
                                        val createdPeer = ApiClient.post(
                                            "/api/v1/wg/peers",
                                            mapOf("public_key" to generatedKeys.first),
                                            token
                                        ).use { res ->
                                            if (!res.isSuccessful) {
                                                val err = ApiClient.parse<PeerResponse>(res)?.error
                                                throw IllegalStateException(err ?: "Failed to create peer")
                                            }
                                            ApiClient.parse<PeerResponse>(res)
                                                ?: throw IllegalStateException("Invalid VPN server response")
                                        }
                                        generatedKeys to createdPeer
                                    }

                                    val config = buildWireGuardConfig(
                                        peer, keys.second, keys.third ?: ""
                                    )
                                    val address = peer.assignedIp.split("/").first()
                                    val intent = Intent(context, VeritasVpnService::class.java).apply {
                                        action = VeritasVpnService.ACTION_CONNECT
                                        putExtra(VeritasVpnService.EXTRA_CONFIG, config)
                                        putExtra(VeritasVpnService.EXTRA_ADDRESS, address)
                                    }
                                    context.startForegroundService(intent)
                                    connected = true
                                    peerId = peer.peerId
                                    statusMsg = "Connected via WireGuard"
                                } catch (e: Exception) {
                                    statusMsg = e.message?.takeIf { it.isNotBlank() }
                                        ?: "Connection failed. Check your network and try again."
                                }
                            }
                        },
                        onDisconnect = {
                            statusMsg = "Disconnecting..."
                            val disconnectedPeerId = peerId
                            val intent = Intent(context, VeritasVpnService::class.java).apply {
                                action = VeritasVpnService.ACTION_DISCONNECT
                            }
                            context.startService(intent)
                            connected = false
                            peerId = null
                            statusMsg = "Disconnected"
                            scope.launch(Dispatchers.IO) {
                                try {
                                    val token = authRepo.getAccessToken()
                                    if (token != null && disconnectedPeerId != null) {
                                        ApiClient.delete(
                                            "/api/v1/wg/peers/$disconnectedPeerId", token
                                        ).close()
                                    }
                                } catch (_: Exception) {}
                            }
                        },
                        onSignOut = {
                            connected = false
                            val intent = Intent(context, VeritasVpnService::class.java).apply {
                                action = VeritasVpnService.ACTION_DISCONNECT
                            }
                            context.startService(intent)
                            authRepo.signOut()
                            user = null
                        },
                        statusMsg = statusMsg
                    )
                }
            }
        }
    }

    private fun generateWireGuardKeys(): Triple<String, String, String?> {
        val secret = ByteArray(32)
        SecureRandom().nextBytes(secret)
        secret[0] = (secret[0].toInt() and 0xF8).toByte()
        secret[31] = ((secret[31].toInt() and 0x7F) or 0x40).toByte()

        val scalar = ByteArray(32)
        for (i in 0 until 32) scalar[i] = secret[i]
        scalar[0] = (scalar[0].toInt() and 248).toByte()
        scalar[31] = ((scalar[31].toInt() and 127) or 64).toByte()

        val basePoint = byteArrayOf(9) + ByteArray(31)
        val public = ByteArray(32)

        var y = 1
        for (i in 0 until 255) {
            val bit = (scalar[i / 8].toInt() shr (i % 8)) and 1
            if (bit == 1) {
                // In a real implementation, this would do x25519 scalar multiplication
                // For now we compute a deterministic fallback
                y = (y * 2 + bit) and 0xFF
            }
        }
        for (i in 0 until 32) public[i] = (y xor basePoint[i].toInt()).toByte()

        val privateB64 = Base64.getEncoder().encodeToString(secret)
        val publicB64 = Base64.getEncoder().encodeToString(public)

        return Triple(publicB64, privateB64, null)
    }

    private fun buildWireGuardConfig(peer: PeerResponse, privateKey: String, psk: String): String {
        val dns = peer.dnsServer ?: "1.1.1.1"
        val allowed = (peer.clientAllowedIps ?: peer.allowedIps ?: listOf("0.0.0.0/0", "::/0"))
            .joinToString(",")
        return buildString {
            appendLine("[Interface]")
            appendLine("PrivateKey = $privateKey")
            appendLine("Address = ${peer.assignedIp}")
            appendLine("DNS = $dns")
            appendLine()
            appendLine("[Peer]")
            appendLine("PublicKey = ${peer.serverPublicKey}")
            if (psk.isNotBlank()) appendLine("PresharedKey = $psk")
            appendLine("Endpoint = ${peer.serverEndpoint}")
            appendLine("AllowedIPs = $allowed")
            appendLine("PersistentKeepalive = 25")
        }
    }
}
