package cloud.veritasvpn.api

import com.google.gson.Gson
import com.google.gson.annotations.SerializedName
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException

object ApiClient {
    private const val BASE_URL = "https://api.veritasvpn.cloud"
    private val JSON = "application/json; charset=utf-8".toMediaType()
    private val client = OkHttpClient.Builder()
        .addInterceptor { chain ->
            val req = chain.request().newBuilder()
                .header("Content-Type", "application/json")
                .build()
            chain.proceed(req)
        }
        .build()
    private val gson = Gson()

    fun post(path: String, body: Map<String, Any>, token: String? = null): Response {
        val b = gson.toJson(body).toRequestBody(JSON)
        val builder = Request.Builder().url("$BASE_URL$path").post(b)
        token?.let { builder.header("Authorization", "Bearer $it") }
        return client.newCall(builder.build()).execute()
    }

    fun delete(path: String, token: String): Response {
        val builder = Request.Builder().url("$BASE_URL$path").delete()
            .header("Authorization", "Bearer $token")
        return client.newCall(builder.build()).execute()
    }

    inline fun <reified T> parse(response: Response): T? {
        val body = response.body?.string() ?: return null
        return try { gson.fromJson(body, T::class.java) } catch (_: Exception) { null }
    }
}

data class AuthResponse(
    @SerializedName("access_token") val accessToken: String,
    @SerializedName("refresh_token") val refreshToken: String,
    @SerializedName("account_id") val accountId: String,
    @SerializedName("expires_at") val expiresAt: Long,
    val email: String? = null
)

data class AuthError(val error: String)

data class PeerResponse(
    @SerializedName("peer_id") val peerId: String,
    @SerializedName("server_public_key") val serverPublicKey: String,
    @SerializedName("server_endpoint") val serverEndpoint: String,
    @SerializedName("assigned_ip") val assignedIp: String,
    @SerializedName("dns_server") val dnsServer: String?,
    @SerializedName("preshared_key") val presharedKey: String?,
    @SerializedName("client_allowed_ips") val clientAllowedIps: List<String>?,
    @SerializedName("allowed_ips") val allowedIps: List<String>?,
    val error: String? = null
)
