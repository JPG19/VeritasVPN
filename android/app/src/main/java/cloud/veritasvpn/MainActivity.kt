package cloud.veritasvpn

import android.Manifest
import android.app.Activity
import android.app.NotificationManager
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.*
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import cloud.veritasvpn.api.ApiClient
import cloud.veritasvpn.api.PeerResponse
import cloud.veritasvpn.auth.AuthRepository
import cloud.veritasvpn.ui.AuthScreen
import cloud.veritasvpn.ui.DashboardScreen
import cloud.veritasvpn.ui.theme.VeritasVPNTheme
import cloud.veritasvpn.vpn.VeritasVpnService
import com.wireguard.crypto.KeyPair
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

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
                var connecting by remember { mutableStateOf(false) }
                var statusMsg by remember { mutableStateOf<String?>(null) }
                val context = LocalContext.current
                val scope = rememberCoroutineScope()

                val vpnPermissionLauncher = rememberLauncherForActivityResult(
                    ActivityResultContracts.StartActivityForResult()
                ) { result ->
                    if (result.resultCode == Activity.RESULT_OK) {
                        statusMsg = null
                        startConnection(
                            context, scope,
                            setStatus = { msg -> statusMsg = msg },
                            setConnecting = { connecting = it }
                        )
                    } else {
                        connecting = false
                        statusMsg = "VPN permission not granted."
                    }
                }

                val notificationPermissionLauncher = rememberLauncherForActivityResult(
                    ActivityResultContracts.RequestPermission()
                ) { }

                DisposableEffect(context) {
                    val receiver = object : BroadcastReceiver() {
                        override fun onReceive(context: Context?, intent: Intent?) {
                            if (intent?.action != VeritasVpnService.ACTION_STATE) return
                            connected = intent.getBooleanExtra(VeritasVpnService.EXTRA_CONNECTED, false)
                            connecting = false
                            statusMsg = if (connected) {
                                val egressIp = intent.getStringExtra(
                                    VeritasVpnService.EXTRA_EGRESS_IP
                                )
                                if (egressIp.isNullOrBlank()) "Connected via WireGuard"
                                else "Connected · VPN egress $egressIp"
                            } else {
                                val error = intent.getStringExtra(VeritasVpnService.EXTRA_ERROR)
                                if (error != null) {
                                    val failedPeerId = peerIdForDisconnect()
                                    if (failedPeerId != null) {
                                        scope.launch(Dispatchers.IO) {
                                            runCatching {
                                                val token = authRepo.getAccessToken()
                                                    ?: return@runCatching
                                                ApiClient.delete(
                                                    "/api/v1/wg/peers/$failedPeerId", token
                                                ).close()
                                            }
                                        }
                                    }
                                }
                                error ?: "Disconnected"
                            }
                        }
                    }
                    ContextCompat.registerReceiver(
                        context,
                        receiver,
                        IntentFilter(VeritasVpnService.ACTION_STATE),
                        ContextCompat.RECEIVER_NOT_EXPORTED
                    )
                    onDispose { context.unregisterReceiver(receiver) }
                }

                fun requestConnect() {
                    if (connecting || connected) return
                    connecting = true
                    VpnService.prepare(context)?.let { consentIntent ->
                        vpnPermissionLauncher.launch(consentIntent)
                        return
                    }
                    if (Build.VERSION.SDK_INT >= 33) {
                        val nm = context.getSystemService(NotificationManager::class.java)
                        if (nm != null && !nm.areNotificationsEnabled()) {
                            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
                        }
                    }
                    startConnection(
                        context, scope,
                        setStatus = { msg -> statusMsg = msg },
                        setConnecting = { connecting = it }
                    )
                }

                if (user == null) {
                    AuthScreen(onAuthenticated = {
                        user = authRepo.getStoredUser()
                    })
                } else {
                    DashboardScreen(
                        connected = connected,
                        connecting = connecting,
                        onConnect = { requestConnect() },
                        onDisconnect = {
                            statusMsg = "Disconnecting..."
                            val disconnectedPeerId = peerIdForDisconnect()
                            val intent = Intent(context, VeritasVpnService::class.java).apply {
                                action = VeritasVpnService.ACTION_DISCONNECT
                            }
                            context.startService(intent)
                            connected = false
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
                            val intent = Intent(context, VeritasVpnService::class.java).apply {
                                action = VeritasVpnService.ACTION_DISCONNECT
                            }
                            context.startService(intent)
                            connected = false
                            authRepo.signOut()
                            user = null
                        },
                        statusMsg = statusMsg
                    )
                }
            }
        }
    }

    private var currentPeerId: String? = null

    private fun peerIdForDisconnect(): String? {
        val id = currentPeerId
        currentPeerId = null
        return id
    }

    private fun startConnection(
        context: Context,
        scope: CoroutineScope,
        setStatus: (String) -> Unit,
        setConnecting: (Boolean) -> Unit
    ) {
        if (currentPeerId != null) return
        setStatus("Connecting...")
        scope.launch {
            try {
                val (keyPair, peer) = withContext(Dispatchers.IO) {
                    authRepo.refreshSession()
                    val token = authRepo.getAccessToken()
                        ?: throw IllegalStateException("Not signed in")
                    val generated = KeyPair()
                    val createdPeer = ApiClient.post(
                        "/api/v1/wg/peers",
                        mapOf("public_key" to generated.publicKey.toBase64()),
                        token
                    ).use { res ->
                        if (!res.isSuccessful) {
                            val err = ApiClient.parse<PeerResponse>(res)?.error
                            throw IllegalStateException(err ?: "Failed to create peer")
                        }
                        ApiClient.parse<PeerResponse>(res)
                            ?: throw IllegalStateException("Invalid VPN server response")
                    }
                    generated to createdPeer
                }

                val config = buildWireGuardConfig(peer, keyPair)
                val intent = Intent(context, VeritasVpnService::class.java).apply {
                    action = VeritasVpnService.ACTION_CONNECT
                    putExtra(VeritasVpnService.EXTRA_CONFIG, config)
                }
                currentPeerId = peer.peerId
                context.startForegroundService(intent)
            } catch (e: Exception) {
                setConnecting(false)
                setStatus(e.message?.takeIf { it.isNotBlank() }
                    ?: "Connection failed. Check your network and try again.")
            }
        }
    }

    private fun buildWireGuardConfig(peer: PeerResponse, keyPair: KeyPair): String {
        val dns = peer.dnsServer ?: "1.1.1.1"
        val allowed = (peer.clientAllowedIps ?: peer.allowedIps ?: listOf("0.0.0.0/0", "::/0"))
            .joinToString(",")
        return buildString {
            appendLine("[Interface]")
            appendLine("PrivateKey = ${keyPair.privateKey.toBase64()}")
            appendLine("Address = ${peer.assignedIp}")
            appendLine("DNS = $dns")
            appendLine("MTU = 1420")
            appendLine()
            appendLine("[Peer]")
            appendLine("PublicKey = ${peer.serverPublicKey}")
            if (!peer.presharedKey.isNullOrEmpty()) {
                appendLine("PresharedKey = ${peer.presharedKey}")
            }
            appendLine("Endpoint = ${peer.serverEndpoint}")
            appendLine("AllowedIPs = $allowed")
            appendLine("PersistentKeepalive = 25")
        }
    }
}
