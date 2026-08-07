package cloud.veritasvpn.auth

import android.content.Context
import android.content.SharedPreferences
import cloud.veritasvpn.api.ApiClient
import cloud.veritasvpn.api.AuthResponse

data class User(
    val email: String? = null,
    val accountId: String,
    val isAnonymous: Boolean = false
)

class AuthRepository(context: Context) {
    private val prefs: SharedPreferences =
        context.getSharedPreferences("veritas_auth", Context.MODE_PRIVATE)

    fun getStoredUser(): User? {
        val id = prefs.getString("account_id", null) ?: return null
        return User(
            email = prefs.getString("email", null),
            accountId = id,
            isAnonymous = prefs.getBoolean("is_anonymous", false)
        )
    }

    fun getAccessToken(): String? = prefs.getString("access_token", null)
    fun getRefreshToken(): String? = prefs.getString("refresh_token", null)

    private fun persist(user: User, data: AuthResponse) {
        prefs.edit()
            .putString("account_id", data.accountId)
            .putString("email", data.email)
            .putBoolean("is_anonymous", user.isAnonymous)
            .putString("access_token", data.accessToken)
            .putString("refresh_token", data.refreshToken)
            .apply()
    }

    fun signIn(email: String, password: String): User {
        val normalized = email.trim().lowercase()
        val res = ApiClient.post("/api/v1/auth/signin",
            mapOf("email" to normalized, "password" to password))
        if (!res.isSuccessful) throw Error(extractError(res))
        val data = ApiClient.parse<AuthResponse>(res)!!
        val user = User(email = data.email ?: normalized, accountId = data.accountId)
        persist(user, data)
        return user
    }

    fun signUp(email: String, password: String): User {
        val normalized = email.trim().lowercase()
        val res = ApiClient.post("/api/v1/auth/register",
            mapOf("email" to normalized, "password" to password))
        if (!res.isSuccessful) throw Error(extractError(res))
        val data = ApiClient.parse<AuthResponse>(res)!!
        val user = User(email = normalized, accountId = data.accountId)
        persist(user, data)
        return user
    }

    fun signInWithAccountId(accountId: String): User {
        val res = ApiClient.post("/api/v1/auth/signin-account",
            mapOf("account_id" to accountId.trim()))
        if (!res.isSuccessful) throw Error(extractError(res))
        val data = ApiClient.parse<AuthResponse>(res)!!
        val user = User(accountId = data.accountId, isAnonymous = true)
        persist(user, data)
        return user
    }

    fun registerAnonymous(): User {
        val res = ApiClient.post("/api/v1/auth/register-anonymous", emptyMap())
        if (!res.isSuccessful) throw Error(extractError(res))
        val data = ApiClient.parse<AuthResponse>(res)!!
        val user = User(accountId = data.accountId, isAnonymous = true)
        persist(user, data)
        return user
    }

    fun refreshSession(): Boolean {
        val rt = getRefreshToken() ?: return false
        val res = try {
            ApiClient.post("/api/v1/auth/refresh", mapOf("refresh_token" to rt))
        } catch (_: Exception) { return false }
        if (!res.isSuccessful) return false
        val data = ApiClient.parse<AuthResponse>(res) ?: return false
        prefs.edit().putString("access_token", data.accessToken)
            .putString("refresh_token", data.refreshToken).apply()
        return true
    }

    fun signOut() {
        prefs.edit().clear().apply()
    }

    private fun extractError(res: okhttp3.Response): String {
        val err = ApiClient.parse<cloud.veritasvpn.api.AuthError>(res)
        val msg = err?.error ?: "Request failed (${res.code})"
        return when {
            msg.contains("incorrect email or password", true) -> "Incorrect email or password."
            msg.contains("password", true) && msg.contains("6") -> "Password must be at least 6 characters."
            msg.contains("already exists", true) -> "An account with this email already exists."
            msg.contains("account", true) -> "Account ID not found."
            else -> msg
        }
    }

    class Error(msg: String) : Exception(msg)
}
