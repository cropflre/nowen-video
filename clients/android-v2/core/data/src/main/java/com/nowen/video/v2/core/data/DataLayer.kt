package com.nowen.video.v2.core.data

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.*
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import java.io.IOException
import java.security.KeyStore
import java.util.UUID
import java.util.concurrent.TimeUnit
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Query

private val Context.sessionDataStore by preferencesDataStore(name = "nowen_v2_session")
private const val PLACEHOLDER_HOST = "placeholder.invalid"

object UrlNormalizer {
    fun normalize(input: String): String? {
        val raw = input.trim().trimEnd('/')
        if (raw.isBlank()) return null
        val withScheme = if (raw.startsWith("http://") || raw.startsWith("https://")) raw else "http://$raw"
        val parsed = withScheme.toHttpUrlOrNull() ?: return null
        if (parsed.host.isBlank()) return null
        return parsed.newBuilder().query(null).fragment(null).build().toString().trimEnd('/')
    }

    fun apiUrl(baseUrl: String, relativePath: String): String? {
        val base = normalize(baseUrl)?.toHttpUrlOrNull() ?: return null
        return base.newBuilder().addPathSegments(relativePath.trimStart('/')).build().toString()
    }
}

@Singleton
class CredentialVault @Inject constructor(
    @ApplicationContext context: Context,
) {
    private val preferences = context.getSharedPreferences("nowen_v2_credentials", Context.MODE_PRIVATE)
    private val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }

    private fun key(): SecretKey {
        val existing = keyStore.getKey(KEY_ALIAS, null) as? SecretKey
        if (existing != null) return existing
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(
                KeyGenParameterSpec.Builder(
                    KEY_ALIAS,
                    KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
                )
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .build(),
            )
            generateKey()
        }
    }

    fun saveToken(serverId: String, token: String) {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val payload = cipher.iv + cipher.doFinal(token.toByteArray(Charsets.UTF_8))
        preferences.edit().putString(serverKey(serverId), Base64.encodeToString(payload, Base64.NO_WRAP)).apply()
    }

    fun readToken(serverId: String): String? = runCatching {
        val encoded = preferences.getString(serverKey(serverId), null) ?: return null
        val payload = Base64.decode(encoded, Base64.NO_WRAP)
        if (payload.size <= IV_SIZE) return null
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(
            Cipher.DECRYPT_MODE,
            key(),
            GCMParameterSpec(128, payload.copyOfRange(0, IV_SIZE)),
        )
        String(cipher.doFinal(payload.copyOfRange(IV_SIZE, payload.size)), Charsets.UTF_8)
    }.getOrElse {
        clearToken(serverId)
        null
    }

    fun clearToken(serverId: String) {
        preferences.edit().remove(serverKey(serverId)).apply()
    }

    private fun serverKey(serverId: String) = "token_$serverId"

    private companion object {
        const val KEY_ALIAS = "nowen_video_v2_session"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_SIZE = 12
    }
}

@Singleton
class ServerSessionStore @Inject constructor(
    @ApplicationContext private val context: Context,
    private val vault: CredentialVault,
    private val json: Json,
) {
    private val _snapshot = MutableStateFlow(SessionSnapshot())
    val snapshot: StateFlow<SessionSnapshot> = _snapshot
    private var accounts: Map<String, UserProfile> = emptyMap()

    suspend fun bootstrap() {
        val prefs = context.sessionDataStore.data.first()
        val servers = decodeServers(prefs[KEY_SERVERS])
        accounts = decodeAccounts(prefs[KEY_ACCOUNTS])
        val activeId = prefs[KEY_ACTIVE_SERVER].takeIf { id -> servers.any { it.id == id } }
        val token = activeId?.let(vault::readToken)
        val user = activeId?.let(accounts::get).takeIf { token != null }
        _snapshot.value = SessionSnapshot(
            servers = servers,
            activeServerId = activeId,
            user = user,
            token = token,
            initialized = true,
        )
    }

    suspend fun saveServer(name: String, rawBaseUrl: String): ServerProfile {
        val normalized = requireNotNull(UrlNormalizer.normalize(rawBaseUrl)) { "服务器地址无效" }
        val current = _snapshot.value
        val existing = current.servers.firstOrNull { it.baseUrl.equals(normalized, ignoreCase = true) }
        val server = (existing ?: ServerProfile(
            id = UUID.randomUUID().toString(),
            name = name.ifBlank { "Nowen 服务器" },
            baseUrl = normalized,
            allowCleartext = normalized.startsWith("http://"),
        )).copy(
            name = name.ifBlank { existing?.name ?: "Nowen 服务器" },
            baseUrl = normalized,
            allowCleartext = normalized.startsWith("http://"),
        )
        val servers = current.servers.filterNot { it.id == server.id } + server
        persistServers(servers, server.id)
        val token = vault.readToken(server.id)
        _snapshot.value = current.copy(
            servers = servers,
            activeServerId = server.id,
            user = accounts[server.id].takeIf { token != null },
            token = token,
        )
        return server
    }

    suspend fun activate(serverId: String) {
        val current = _snapshot.value
        require(current.servers.any { it.id == serverId })
        val token = vault.readToken(serverId)
        context.sessionDataStore.edit { it[KEY_ACTIVE_SERVER] = serverId }
        _snapshot.value = current.copy(
            activeServerId = serverId,
            user = accounts[serverId].takeIf { token != null },
            token = token,
        )
    }

    suspend fun deactivate() {
        context.sessionDataStore.edit { it.remove(KEY_ACTIVE_SERVER) }
        _snapshot.value = _snapshot.value.copy(activeServerId = null, user = null, token = null)
    }

    suspend fun remove(serverId: String) {
        val current = _snapshot.value
        val servers = current.servers.filterNot { it.id == serverId }
        vault.clearToken(serverId)
        accounts = accounts - serverId
        val nextActive = if (current.activeServerId == serverId) null else current.activeServerId
        context.sessionDataStore.edit {
            it[KEY_SERVERS] = json.encodeToString(servers)
            it[KEY_ACCOUNTS] = json.encodeToString(accounts)
            if (nextActive == null) it.remove(KEY_ACTIVE_SERVER) else it[KEY_ACTIVE_SERVER] = nextActive
        }
        _snapshot.value = current.copy(
            servers = servers,
            activeServerId = nextActive,
            user = if (nextActive == current.activeServerId) current.user else null,
            token = if (nextActive == current.activeServerId) current.token else null,
        )
    }

    suspend fun saveAuthenticatedSession(token: String, user: UserProfile) {
        val activeId = requireNotNull(_snapshot.value.activeServerId)
        vault.saveToken(activeId, token)
        accounts = accounts + (activeId to user)
        context.sessionDataStore.edit { it[KEY_ACCOUNTS] = json.encodeToString(accounts) }
        _snapshot.value = _snapshot.value.copy(token = token, user = user)
    }

    suspend fun clearAuthentication() {
        val activeId = _snapshot.value.activeServerId
        if (activeId != null) {
            vault.clearToken(activeId)
            accounts = accounts - activeId
            context.sessionDataStore.edit { it[KEY_ACCOUNTS] = json.encodeToString(accounts) }
        }
        _snapshot.value = _snapshot.value.copy(token = null, user = null)
    }

    private suspend fun persistServers(servers: List<ServerProfile>, activeId: String?) {
        context.sessionDataStore.edit {
            it[KEY_SERVERS] = json.encodeToString(servers)
            if (activeId == null) it.remove(KEY_ACTIVE_SERVER) else it[KEY_ACTIVE_SERVER] = activeId
        }
    }

    private fun decodeServers(raw: String?): List<ServerProfile> =
        raw?.let { runCatching { json.decodeFromString<List<ServerProfile>>(it) }.getOrDefault(emptyList()) }
            ?: emptyList()

    private fun decodeAccounts(raw: String?): Map<String, UserProfile> =
        raw?.let { runCatching { json.decodeFromString<Map<String, UserProfile>>(it) }.getOrDefault(emptyMap()) }
            ?: emptyMap()

    private companion object {
        val KEY_SERVERS = stringPreferencesKey("servers")
        val KEY_ACTIVE_SERVER = stringPreferencesKey("active_server")
        val KEY_ACCOUNTS = stringPreferencesKey("accounts")
    }
}

interface NowenApi {
    @GET("auth/status") suspend fun status(): InitStatusEnvelope
    @POST("auth/login") suspend fun login(@Body request: LoginRequest): TokenResponse
    @POST("auth/refresh") suspend fun refresh(): TokenResponse
    @PUT("auth/password") suspend fun changePassword(@Body request: PasswordChangeRequest): PasswordChangeResponse
    @GET("libraries") suspend fun libraries(): ApiEnvelope<List<LibrarySummary>>
    @GET("media/continue") suspend fun continueWatching(): ApiEnvelope<List<MediaCard>>
    @GET("media/recent/mixed") suspend fun recent(@Query("limit") limit: Int = 20): ApiEnvelope<List<MediaCard>>
    @GET("search/mixed") suspend fun search(@Query("q") query: String): SearchResponse
}

@Serializable
data class SearchResponse(
    val data: List<MediaCard> = emptyList(),
    val media: List<MediaCard> = emptyList(),
    val series: List<MediaCard> = emptyList(),
) {
    fun all(): List<MediaCard> = (data + media + series).distinctBy { it.resolvedId }
}

class SessionInterceptor(
    private val sessionStore: ServerSessionStore,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val snapshot = sessionStore.snapshot.value
        val original = chain.request()
        val server = snapshot.activeServer
        val targetUrl = if (server != null && original.url.host == PLACEHOLDER_HOST) {
            val base = server.baseUrl.toHttpUrlOrNull()
            base?.newBuilder()
                ?.addPathSegments(original.url.encodedPath.trimStart('/'))
                ?.encodedQuery(original.url.encodedQuery)
                ?.build()
                ?: original.url
        } else original.url

        val request = original.newBuilder()
            .url(targetUrl)
            .header("Accept", "application/json")
            .apply {
                snapshot.token?.takeIf { it.isNotBlank() }?.let { header("Authorization", "Bearer $it") }
            }
            .build()
        return chain.proceed(request)
    }
}

@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {
    @Provides
    @Singleton
    fun json(): Json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
        isLenient = true
        explicitNulls = false
        encodeDefaults = true
    }

    @Provides
    @Singleton
    fun okHttp(sessionStore: ServerSessionStore): OkHttpClient =
        OkHttpClient.Builder()
            .addInterceptor(SessionInterceptor(sessionStore))
            .connectTimeout(12, TimeUnit.SECONDS)
            .readTimeout(45, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .retryOnConnectionFailure(true)
            .build()

    @Provides
    @Singleton
    fun api(client: OkHttpClient, json: Json): NowenApi =
        Retrofit.Builder()
            .baseUrl("https://$PLACEHOLDER_HOST/api/")
            .client(client)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(NowenApi::class.java)
}

data class ServerProbe(
    val serverName: String = "Nowen Video",
    val version: String = "",
)

@Singleton
class NowenRepository @Inject constructor(
    private val api: NowenApi,
    private val client: OkHttpClient,
    private val sessionStore: ServerSessionStore,
) {
    suspend fun probe(baseUrl: String): Result<ServerProbe> = runCatching {
        val healthUrl = requireNotNull(UrlNormalizer.apiUrl(baseUrl, "api/health"))
        val directClient = client.newBuilder().apply {
            interceptors().clear()
            networkInterceptors().clear()
        }.build()
        directClient.newCall(Request.Builder().url(healthUrl).header("Accept", "application/json").build())
            .execute()
            .use { if (!it.isSuccessful) error("服务器返回 HTTP ${it.code}") }
        ServerProbe()
    }

    suspend fun login(username: String, password: String): Result<TokenResponse> = apiCall {
        val response = api.login(LoginRequest(username.trim(), password))
        val user = response.user.copy(
            mustChangePassword = response.user.mustChangePassword || response.mustChangePassword,
        )
        sessionStore.saveAuthenticatedSession(response.token, user)
        response.copy(user = user)
    }

    suspend fun changePassword(oldPassword: String, newPassword: String): Result<TokenResponse> = apiCall {
        val response = api.changePassword(PasswordChangeRequest(oldPassword, newPassword))
        val token = requireNotNull(response.data) {
            response.message.ifBlank { "密码已修改，但服务器未返回新会话" }
        }
        sessionStore.saveAuthenticatedSession(token.token, token.user)
        token
    }

    suspend fun loadHome(): Result<HomeContent> = apiCall {
        coroutineScope {
            val libraries = async { api.libraries().data }
            val continuing = async { api.continueWatching().data }
            val recent = async { api.recent(24).data }
            HomeContent(libraries.await(), continuing.await(), recent.await())
        }
    }

    suspend fun search(query: String): Result<List<MediaCard>> = apiCall {
        if (query.isBlank()) emptyList() else api.search(query.trim()).all()
    }

    suspend fun logout() = sessionStore.clearAuthentication()

    private suspend fun <T> apiCall(block: suspend () -> T): Result<T> = runCatching { block() }.recoverCatching {
        when (it) {
            is HttpException -> if (it.code() == 401) throw UnauthorizedException() else throw ServerException(it.code(), it.message())
            is IOException -> throw NetworkException(it.message ?: "网络不可用")
            else -> throw it
        }
    }
}

class UnauthorizedException : IllegalStateException("登录状态已失效")
class NetworkException(message: String) : IOException(message)
class ServerException(val code: Int, message: String) : IOException(message)
