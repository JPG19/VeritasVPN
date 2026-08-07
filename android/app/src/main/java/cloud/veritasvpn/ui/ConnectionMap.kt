package cloud.veritasvpn.ui

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.dp
import cloud.veritasvpn.ui.theme.*

@Composable
fun ConnectionMap(modifier: Modifier = Modifier, connected: Boolean) {
    val infiniteTransition = rememberInfiniteTransition()
    val pulseScale by infiniteTransition.animateFloat(
        initialValue = 1f, targetValue = 1.5f,
        animationSpec = infiniteRepeatable(tween(1500), RepeatMode.Reverse)
    )
    val offset by infiniteTransition.animateFloat(
        initialValue = 0f, targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(2800))
    )

    // Map coordinates relative to 900x430 viewBox
    val originX = 128f; val originY = 125f
    val destX = 599f; val destY = 319f
    val routeCps = listOf(
        Offset(255f, 88f), Offset(339f, 164f),
        Offset(421f, 239f), Offset(535f, 315f)
    )

    Canvas(modifier = modifier.fillMaxWidth().height(200.dp)) {
        val w = size.width
        val h = size.height
        val sx = w / 900f; val sy = h / 430f

        fun Float.x() = this * sx
        fun Float.y() = this * sy
        fun Offset.cs() = Offset(this.x.x(), this.y.y())

        val bgBrush = Brush.horizontalGradient(listOf(DarkSurface, DarkSurface2, DarkSurface))
        drawRect(bgBrush)

        val gridColor = Color.White.copy(alpha = 0.04f)
        for (x in listOf(0f, 225f, 450f, 675f)) {
            drawLine(gridColor, Offset(x.x(), 0f), Offset(x.x(), h))
        }
        for (y in listOf(108f, 215f, 322f)) {
            drawLine(gridColor, Offset(0f, y.y()), Offset(w, y.y()))
        }

        // Route line
        val routePath = Path().apply {
            moveTo(originX.x(), originY.y())
            cubicTo(
                255f.x(), 88f.y(), 339f.x(), 164f.y(),
                421f.x(), 239f.y()
            )
            cubicTo(
                535f.x(), 315f.y(), 599f.x(), 319f.y(),
                599f.x(), 319f.y()
            )
        }
        drawPath(routePath, Color.White.copy(alpha = 0.15f), style = Stroke(width = 2.5f * sx))

        val gradient = Brush.linearGradient(listOf(Purple800, Teal400))
        drawPath(routePath, gradient, style = Stroke(width = 2.5f * sx))

        // Particle
        val particlePos by remember(offset) { mutableStateOf(routePoint(offset, originX, originY, destX, destY, routeCps, sx, sy)) }
        drawCircle(gradient, 5f * sx, Offset(particlePos.first, particlePos.second))

        // Origin dot
        val originPulse = if (connected) 14f * sx * pulseScale else 14f * sx
        drawCircle(gradient.copy(alpha = 0.3f), originPulse, Offset(originX.x(), originY.y()))
        drawCircle(Color.White.copy(alpha = 0.8f), 5f * sx, Offset(originX.x(), originY.y()))

        // Destination dot
        val destPulse = if (connected) 18f * sx * pulseScale else 18f * sx
        drawCircle(gradient.copy(alpha = 0.4f), destPulse, Offset(destX.x(), destY.y()))
        drawCircle(gradient, 8f * sx, Offset(destX.x(), destY.y()))
    }
}

private fun routePoint(
    t: Float, ox: Float, oy: Float, dx: Float, dy: Float,
    cps: List<Offset>, sx: Float, sy: Float
): Pair<Float, Float> {
    val x = cubicBezier(t, ox, cps[0].x, cps[2].x, dx) * sx
    val y = cubicBezier(t, oy, cps[0].y, cps[2].y, dy) * sy
    return Pair(x, y)
}

private fun cubicBezier(t: Float, p0: Float, p1: Float, p2: Float, p3: Float): Float {
    val u = 1f - t
    return u*u*u*p0 + 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t*p3
}
