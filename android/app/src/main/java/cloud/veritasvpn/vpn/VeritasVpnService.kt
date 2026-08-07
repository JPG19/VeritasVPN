package cloud.veritasvpn.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.ParcelFileDescriptor
import android.widget.Toast
import androidx.core.app.NotificationCompat
import cloud.veritasvpn.MainActivity

class VeritasVpnService : VpnService() {
    private var tunFd: ParcelFileDescriptor? = null
    private var ifname: String = "veritas0"

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_CONNECT -> {
                val config = intent.getStringExtra(EXTRA_CONFIG) ?: ""
                val address = intent.getStringExtra(EXTRA_ADDRESS) ?: ""
                connect(config, address)
            }
            ACTION_DISCONNECT -> disconnect()
            Action.STOP -> {
                disconnect()
                stopSelf()
            }
        }
        return START_STICKY
    }

    fun connect(wgConfig: String, address: String) {
        try {
            if (!WireGuardBackend.isAvailable()) {
                Toast.makeText(this, "WireGuard native library not available", Toast.LENGTH_LONG).show()
                return
            }

            val builder = Builder()
            builder.setSession("VeritasVPN")
            builder.addAddress(address, 32)
            builder.addRoute("0.0.0.0", 0)
            builder.addRoute("::", 0)
            builder.addDnsServer("1.1.1.1")
            builder.addDnsServer("8.8.8.8")
            builder.setMtu(1420)
            builder.setBlocking(true)

            tunFd = builder.establish()
            if (tunFd == null) throw Exception("TUN device creation failed")

            val result = WireGuardBackend.wgTurnOn(ifname, tunFd!!.fd, wgConfig)
            if (result != 0) {
                tunFd?.close()
                tunFd = null
                throw Exception("WireGuard backend failed with code $result")
            }

            val notification = buildNotification(true)
            startForeground(NOTIFICATION_ID, notification)
        } catch (e: Exception) {
            Toast.makeText(this, "Connection failed: ${e.message}", Toast.LENGTH_LONG).show()
            disconnect()
        }
    }

    fun disconnect() {
        if (WireGuardBackend.isAvailable()) {
            try { WireGuardBackend.wgTurnOff(ifname) } catch (_: Exception) {}
        }
        try { tunFd?.close() } catch (_: Exception) {}
        tunFd = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun buildNotification(connected: Boolean): Notification {
        val openIntent = Intent(this, MainActivity::class.java).let {
            PendingIntent.getActivity(this, 0, it,
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        }

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("VeritasVPN")
            .setContentText(if (connected) "Connected — Paraguay" else "Not connected")
            .setSmallIcon(android.R.drawable.ic_menu_secure)
            .setContentIntent(openIntent)
            .setOngoing(connected)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID, "VeritasVPN", NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "VPN connection status"
        }
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    override fun onDestroy() {
        disconnect()
        super.onDestroy()
    }

    companion object {
        const val CHANNEL_ID = "veritas_vpn"
        const val NOTIFICATION_ID = 1
        const val ACTION_CONNECT = "cloud.veritasvpn.CONNECT"
        const val ACTION_DISCONNECT = "cloud.veritasvpn.DISCONNECT"
        const val EXTRA_CONFIG = "config"
        const val EXTRA_ADDRESS = "address"
    }
}
