package cloud.veritasvpn.ui

import androidx.compose.animation.*
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import cloud.veritasvpn.ui.theme.*

@Composable
fun DashboardScreen(
    connected: Boolean,
    connecting: Boolean,
    onConnect: () -> Unit,
    onDisconnect: () -> Unit,
    onSignOut: () -> Unit,
    statusMsg: String?
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink)
            .padding(16.dp)
    ) {
        // Header
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(36.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .background(
                            Brush.linearGradient(listOf(Cyan, Cyan))
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Text("V", color = Color.White, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                }
                Spacer(Modifier.width(8.dp))
                Column {
                    Text("VeritasVPN", color = Paper, fontWeight = FontWeight.Bold, fontSize = 16.sp)
                    Text("Privacy is truth.", color = PaperDim, fontSize = 11.sp)
                }
            }
            TextButton(onClick = onSignOut) {
                Text("Sign out", color = PaperDim, fontSize = 13.sp)
            }
        }

        Spacer(Modifier.height(24.dp))

        // Connection Map
        ConnectionMap(connected = connected)

        Spacer(Modifier.height(20.dp))

        // Route labels
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Column {
                Text("Your device", color = PaperDim, fontSize = 12.sp)
                Text("Protected locally", color = PaperMuted, fontSize = 13.sp, fontWeight = FontWeight.Medium)
            }
            Column(horizontalAlignment = Alignment.End) {
                Text("PARAGUAY", color = PaperDim, fontSize = 12.sp)
                Text("Asunción metro", color = PaperMuted, fontSize = 13.sp, fontWeight = FontWeight.Medium)
            }
        }

        Spacer(Modifier.height(32.dp))

        // Status Orb
        Column(
            modifier = Modifier.fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Box(
                modifier = Modifier
                    .size(80.dp)
                    .clip(CircleShape)
                    .background(
                        if (connected) SuccessGreen.copy(alpha = 0.15f)
                        else CardBg
                    ),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = if (connected) "✓" else "V",
                    color = if (connected) SuccessGreen else Cyan,
                    fontWeight = FontWeight.Bold,
                    fontSize = 28.sp
                )
            }

            Spacer(Modifier.height(16.dp))

            Text(
                text = if (connected) "CONNECTION SECURED" else "VPN READY",
                color = PaperDim,
                fontSize = 12.sp,
                letterSpacing = 2.sp
            )
            Text(
                text = if (connected) "You're protected" else "Connect to Veritas",
                color = Paper,
                fontWeight = FontWeight.Bold,
                fontSize = 22.sp
            )
            Text(
                text = if (connected)
                    "Your internet traffic is encrypted through our WireGuard node in Paraguay."
                else "Route your traffic privately through our live node in Paraguay.",
                color = PaperMuted,
                fontSize = 14.sp,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(horizontal = 24.dp)
            )

            Spacer(Modifier.height(16.dp))

            // Server card
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(12.dp))
                    .background(CardBg)
                    .padding(14.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text("🇵🇾", fontSize = 24.sp)
                Spacer(Modifier.width(12.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text("Paraguay", color = Paper, fontWeight = FontWeight.SemiBold)
                    Text("Asunción metro · Automatic", color = PaperDim, fontSize = 12.sp)
                }
                Box(
                    modifier = Modifier
                        .clip(RoundedCornerShape(4.dp))
                        .background(SuccessGreen.copy(alpha = 0.15f))
                        .padding(horizontal = 8.dp, vertical = 3.dp)
                ) {
                    Text("LIVE", color = SuccessGreen, fontSize = 11.sp, fontWeight = FontWeight.Bold)
                }
            }

            Spacer(Modifier.height(24.dp))

            // Connect / Disconnect button
            Button(
                onClick = if (connected) onDisconnect else onConnect,
                enabled = !connecting,
                modifier = Modifier.fillMaxWidth().height(52.dp),
                shape = RoundedCornerShape(26.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = if (connected) ErrorRed else Cyan
                )
            ) {
                Text(
                    text = when {
                        connecting -> "Connecting..."
                        connected -> "Disconnect"
                        else -> "Connect now"
                    },
                    color = Color.White,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 16.sp
                )
            }

            // Connection status
            Row(
                modifier = Modifier.padding(top = 12.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Box(
                    modifier = Modifier
                        .size(8.dp)
                        .clip(CircleShape)
                        .background(if (connected) SuccessGreen else PaperDim)
                )
                Spacer(Modifier.width(6.dp))
                Text(
                    text = if (connected) "Protected · WireGuard" else "Not connected",
                    color = PaperDim,
                    fontSize = 12.sp
                )
            }

            // Status message
            statusMsg?.let { msg ->
                Spacer(Modifier.height(8.dp))
                Text(
                    text = msg,
                    color = if (connected) SuccessGreen else WarningOrange,
                    fontSize = 12.sp,
                    textAlign = TextAlign.Center
                )
            }
        }
    }
}
