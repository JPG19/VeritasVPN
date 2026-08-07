package cloud.veritasvpn.ui.theme

import androidx.compose.ui.graphics.Color

// Veritas brand palette (matches website/css/style.css)
// Gradient: cyan -> royal on charcoal ink

val Cyan = Color(0xFF00D2FF)
val CyanHover = Color(0xFF33DBFF)
val CyanSoft = Color(0x2400D2FF) // rgba(0, 210, 255, 0.14)
val Royal = Color(0xFF0047FF)
val RoyalHover = Color(0xFF1A5CFF)
val BlueDeep = Color(0xFF002984)

val Ink = Color(0xFF05070A)
val Ink2 = Color(0xFF0B1018)
val Ink3 = Color(0xFF121820)
val CardBg = Color(0xFF0F1520)

val Paper = Color(0xFFFFFFFF)
val PaperMuted = Color(0xFF9AA8BC)
val PaperDim = Color(0xFF6B7A90)

val Line = Color(0x1AFFFFFF)      // rgba(255, 255, 255, 0.10)
val LineStrong = Color(0x2EFFFFFF) // rgba(255, 255, 255, 0.18)

val SuccessGreen = Cyan
val ErrorRed = Color(0xFFF0A0A0)
val WarningOrange = Color(0xFFFFB74D)

val GradientCyanToRoyal = listOf(Cyan, Royal)
val GradientDarkToCyan = listOf(Ink, BlueDeep)
