package cloud.veritasvpn.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import cloud.veritasvpn.ui.theme.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

enum class AuthMode { SIGN_IN, SIGN_UP }
enum class AuthMethod { EMAIL, ACCOUNT_ID }

@Composable
fun AuthScreen(
    onAuthenticated: () -> Unit
) {
    var mode by remember { mutableStateOf(AuthMode.SIGN_IN) }
    var method by remember { mutableStateOf(AuthMethod.EMAIL) }
    var email by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var accountId by remember { mutableStateOf("") }
    var newAccountId by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var loading by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    val context = androidx.compose.ui.platform.LocalContext.current
    val authRepo = remember(context) { cloud.veritasvpn.auth.AuthRepository(context) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink)
            .verticalScroll(rememberScrollState())
            .padding(24.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Spacer(Modifier.height(60.dp))

        // Brand
        Text(
            text = "VeritasVPN",
            style = MaterialTheme.typography.displayMedium,
            fontWeight = FontWeight.Bold,
            color = Paper
        )
        Text("Privacy is truth.", color = PaperDim, fontSize = 14.sp)

        Spacer(Modifier.height(32.dp))

        if (newAccountId != null) {
            // Show new account ID
            Card(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(12.dp),
                colors = CardDefaults.cardColors(containerColor = CardBg)
            ) {
                Column(
                    modifier = Modifier.padding(20.dp),
                    horizontalAlignment = Alignment.CenterHorizontally
                ) {
                    Text(
                        "Your Account ID",
                        color = SuccessGreen,
                        fontWeight = FontWeight.SemiBold,
                        fontSize = 13.sp
                    )
                    Text(
                        "Copy it now — it cannot be recovered:",
                        color = PaperDim,
                        fontSize = 12.sp
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        newAccountId!!,
                        color = Paper,
                        fontWeight = FontWeight.Bold,
                        fontSize = 18.sp,
                        textAlign = TextAlign.Center
                    )
                    Spacer(Modifier.height(16.dp))
                    Button(
                        onClick = { onAuthenticated() },
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(24.dp),
                        colors = ButtonDefaults.buttonColors(containerColor = Cyan)
                    ) { Text("Continue") }
                }
            }
            return
        }

        // Auth tabs
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.Center
        ) {
            TabButton(selected = mode == AuthMode.SIGN_IN, onClick = { mode = AuthMode.SIGN_IN }, text = "Sign in")
            Spacer(Modifier.width(8.dp))
            TabButton(selected = mode == AuthMode.SIGN_UP, onClick = { mode = AuthMode.SIGN_UP }, text = "Sign up")
        }

        Spacer(Modifier.height(20.dp))

        // Error
        error?.let {
            Text(it, color = ErrorRed, fontSize = 13.sp, modifier = Modifier.padding(bottom = 12.dp))
        }

        // Form
        if (method == AuthMethod.EMAIL) {
            OutlinedTextField(
                value = email, onValueChange = { email = it },
                label = { Text("Email") },
                modifier = Modifier.fillMaxWidth(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email),
                colors = inputColors(),
                singleLine = true
            )
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = password, onValueChange = { password = it },
                label = { Text("Password") },
                modifier = Modifier.fillMaxWidth(),
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                colors = inputColors(),
                singleLine = true
            )
        } else if (mode == AuthMode.SIGN_IN) {
            OutlinedTextField(
                value = accountId, onValueChange = { accountId = it },
                label = { Text("Account ID") },
                modifier = Modifier.fillMaxWidth(),
                colors = inputColors(),
                singleLine = true
            )
        }

        Spacer(Modifier.height(16.dp))

        Button(
            onClick = {
                error = null
                val validationError = when {
                    method == AuthMethod.EMAIL && email.isBlank() -> "Enter your email address."
                    method == AuthMethod.EMAIL && password.isBlank() -> "Enter your password."
                    method == AuthMethod.ACCOUNT_ID && mode == AuthMode.SIGN_IN && accountId.isBlank() ->
                        "Enter your Account ID."
                    else -> null
                }
                if (validationError != null) {
                    error = validationError
                } else {
                    loading = true
                    scope.launch {
                        try {
                            val user = withContext(Dispatchers.IO) {
                                when {
                                    method == AuthMethod.EMAIL && mode == AuthMode.SIGN_IN ->
                                        authRepo.signIn(email, password)
                                    method == AuthMethod.EMAIL && mode == AuthMode.SIGN_UP ->
                                        authRepo.signUp(email, password)
                                    method == AuthMethod.ACCOUNT_ID && mode == AuthMode.SIGN_IN ->
                                        authRepo.signInWithAccountId(accountId)
                                    else -> authRepo.registerAnonymous()
                                }
                            }
                            if (method == AuthMethod.ACCOUNT_ID && mode == AuthMode.SIGN_UP) {
                                newAccountId = user.accountId
                            } else {
                                onAuthenticated()
                            }
                        } catch (e: Exception) {
                            error = e.message?.takeIf { it.isNotBlank() }
                                ?: "Sign in failed. Check your connection and try again."
                        } finally {
                            loading = false
                        }
                    }
                }
            },
            modifier = Modifier.fillMaxWidth().height(50.dp),
            shape = RoundedCornerShape(25.dp),
            colors = ButtonDefaults.buttonColors(containerColor = Cyan),
            enabled = !loading
        ) {
            Text(
                if (loading) "Please wait..."
                else if (mode == AuthMode.SIGN_IN) "Sign in"
                else if (method == AuthMethod.ACCOUNT_ID) "Create anonymous account"
                else "Create account",
                color = Color.White,
                fontWeight = FontWeight.SemiBold
            )
        }

        Spacer(Modifier.height(16.dp))

        // Switch method
        TextButton(onClick = {
            error = null
            method = if (method == AuthMethod.EMAIL) AuthMethod.ACCOUNT_ID else AuthMethod.EMAIL
            if (method == AuthMethod.ACCOUNT_ID && mode == AuthMode.SIGN_UP) mode = AuthMode.SIGN_IN
        }) {
            Text(
                text = if (method == AuthMethod.EMAIL)
                    if (mode == AuthMode.SIGN_IN) "Sign in with Account ID instead"
                    else "Skip email — create anonymous account"
                else "Use email instead",
                color = PaperDim,
                fontSize = 13.sp
            )
        }

        if (mode == AuthMode.SIGN_UP && method == AuthMethod.ACCOUNT_ID) {
            Text(
                "Creates an anonymous account. You'll get an Account ID to save — no email required.",
                color = PaperDim, fontSize = 12.sp, textAlign = TextAlign.Center,
                modifier = Modifier.padding(top = 4.dp)
            )
        }
    }
}

@Composable
private fun TabButton(selected: Boolean, onClick: () -> Unit, text: String) {
    TextButton(
        onClick = onClick,
        colors = ButtonDefaults.textButtonColors(
            contentColor = if (selected) Color.White else PaperDim
        ),
        modifier = Modifier
            .background(
                if (selected) Cyan.copy(alpha = 0.3f) else CardBg,
                RoundedCornerShape(20.dp)
            )
    ) {
        Text(text, fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal)
    }
}

@Composable
private fun inputColors() = OutlinedTextFieldDefaults.colors(
    focusedTextColor = Paper,
    unfocusedTextColor = Paper,
    focusedBorderColor = Cyan,
    unfocusedBorderColor = LineStrong,
    cursorColor = Cyan,
    focusedLabelColor = Cyan,
    unfocusedLabelColor = PaperDim
)
