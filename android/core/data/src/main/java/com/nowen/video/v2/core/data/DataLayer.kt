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
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException
import java.security.KeyStore
import java.util.UUID
import java.util.concurrent.TimeUnit
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.inject.Inject
import javax.inject.Singleton
import javax.net.ssl.SSLException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.withContext
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Query

private val Context.sessionDataStore by preferencesDataStore(name = "nowen_v2_session")
internal const val PLACEHOLDER_HOST = "placeholder.invalid"

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
    private var expirations: Map<String, Long> = emptyMap()

    suspend fun bootstrap() {
        val prefs = context.sessionDataStore.data.first()
        val servers = decodeServers(prefs[KEY_SERVERS])
        accounts = decodeAccounts(prefs[KEY_ACCOUNTS])
        expirations = decodeExpirations(prefs[KEY_EXPIRATIONS])
        val activeId = prefs[KEY_ACTIVE_SERVER].takeIf { id -> servers.any { it.id == id } }
        val restored = activeId?.let { restoreAuthentication(it) }
        _snapshot.value = SessionSnapshot(
            servers = servers,
            activeServerId = activeId,
            user = restored?.second,
            token = restored?.first,
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
        val restored = restoreAuthentication(server.id)
        _snapshot.value = current.copy(
            servers = servers,
            activeServerId = server.id,
            user = restored.second,
            token = restored.first,
        )
        return server
    }

    suspend fun activate(serverId: String) {
        val current = _snapshot.value
        require(current.servers.any { it.id == serverId })
        val restored = restoreAuthentication(serverId)
        context.sessionDataStore.edit { it[KEY_ACTIVE_SERVER] = serverId }
        _snapshot.value = current.copy(
            activeServerId = serverId,
            user = restored.second,
            token = restored.first,
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
        expirations = expirations - serverId
        val nextActive = if (current.activeServerId == serverId) null else current.activeServerId
        context.sessionDataStore.edit {
            it[KEY_SERVERS] = json.encodeToString(servers)
            it[KEY_ACCOUNTS] = json.encodeToString(accounts)
            it[KEY_EXPIRATIONS] = json.encodeToString(expirations)
            if (nextActive == null) it.remove(KEY_ACTIVE_SERVER) else it[KEY_ACTIVE_SERVER] = nextActive
        }
        _snapshot.value = current.copy(
            servers = servers,
            activeServerId = nextActive,
            user = if (nextActive == current.activeServerId) current.user else null,
            token = if (nextActive == current.activeServerId) current.token else null,
        )
    }

    suspend fun saveAuthenticatedSession(
        token: String,
        user: UserProfile,
        expiresAtEpochSeconds: Long = 0L,
    ) {
        val activeId = requireNotNull(_snapshot.value.activeServerId)
        vault.saveToken(activeId, token)
        accounts = accounts + (activeId to user)
        expirations = if (expiresAtEpochSeconds > 0L) {
            expirations + (activeId to expiresAtEpochSeconds)
        } else {
            expirations - activeId
        }
        context.sessionDataStore.edit {
            it[KEY_ACCOUNTS] = json.encodeToString(accounts)
            it[KEY_EXPIRATIONS] = json.encodeToString(expirations)
        }
        _snapshot.value = _snapshot.value.copy(token = token, user = user)
    }

    suspend fun clearAuthentication() {
        val activeId = _snapshot.value.activeServerId
        if (activeId != null) clearPersistedAuthentication(activeId)
        _snapshot.value = _snapshot.value.copy(token = null, user = null)
    }

    fun expiresAtEpochSeconds(serverId: String? = _snapshot.value.activeServerId): Long =
        serverId?.let(expirations::get) ?: 0L

    private suspend fun restoreAuthentication(serverId: String): Pair<String?, UserProfile?> {
        val token = vault.readToken(serverId) ?: return null to null
        val expiresAt = expirations[serverId] ?: 0L
        if (expiresAt > 0L && expiresAt <= currentEpochSeconds()) {
            clearPersistedAuthentication(serverId)
            return null to null
        }
        return token to accounts[serverId]
    }

    private suspend fun clearPersistedAuthentication(serverId: String) {
        vault.clearToken(serverId)
        accounts = accounts - serverId
        expirations = expirations - serverId
        context.sessionDataStore.edit {
            it[KEY_ACCOUNTS] = json.encodeToString(accounts)
            it[KEY_EXPIRATIONS] = json.encodeToString(expirations)
        }
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

    private fun decodeExpirations(raw: String?): Map<String, Long> =
        raw?.let { runCatching { json.decodeFromString<Map<String, Long>>(it) }.getOrDefault(emptyMap()) }
            ?: emptyMap()

    private companion object {
        val KEY_SERVERS = stringPreferencesKey("servers")
        val KEY_ACTIVE_SERVER = stringPreferencesKey("active_server")
        val KEY_ACCOUNTS = stringPreferencesKey("accounts")
        val KEY_EXPIRATIONS = stringPreferencesKey("token_expirations")
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
    fun okHttp(
        sessionStore: ServerSessionStore,
        refreshCoordinator: SessionRefreshCoordinator,
    ): OkHttpClient =
        OkHttpClient.Builder()
            .addInterceptor(SessionInterceptor(sessionStore, refreshCoordinator))
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
    private val json: Json,
) {
    suspend fun probe(baseUrl: String): Result<ServerProbe> = runCatching {
        withContext(Dispatchers.IO) {
            val directClient = client.newBuilder().apply {
                interceptors().clear()
                networkInterceptors().clear()
            }.build()
            directClient.newCall(buildServerHandshakeRequest(baseUrl))
                .execute()
                .use { response ->
                    val handshake = response.readServerHandshake(json)
                    ServerProbe(
                        serverName = handshake.serverName,
                        version = handshake.version,
                    )
                }
        }
    }.recoverCatching { error ->
        throw error.asConnectionFailure()
    }

    suspend fun login(username: String, password: String): Result<TokenResponse> = apiCall {
        val response = api.login(LoginRequest(username.trim(), password))
        val user = response.user.copy(
            mustChangePassword = response.user.mustChangePassword || response.mustChangePassword,
        )
        sessionStore.saveAuthenticatedSession(response.token, user, response.expiresAt)
        response.copy(user = user)
    }

    suspend fun changePassword(oldPassword: String, newPassword: String): Result<TokenResponse> = apiCall {
        val response = api.changePassword(PasswordChangeRequest(oldPassword, newPassword))
        val token = requireNotNull(response.data) {
            response.message.ifBlank { "密码已修改，但服务器未返回新会话" }
        }
        sessionStore.saveAuthenticatedSession(token.token, token.user, token.expiresAt)
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

internal fun currentEpochSeconds(): Long = System.currentTimeMillis() / 1_000L

private fun Throwable.asConnectionFailure(): Throwable = when (this) {
    is IllegalArgumentException -> this
    is SocketTimeoutException -> IOException("连接超时，请确认服务器已启动，并且手机与服务器位于同一局域网", this)
    is UnknownHostException -> IOException("无法解析服务器地址，请检查域名或 IP 地址", this)
    is ConnectException -> IOException("无法连接服务器，请确认服务正在运行且端口可以访问", this)
    is SSLException -> IOException("HTTPS 连接失败，请检查证书，或确认服务器是否应使用 HTTP", this)
    is IOException -> when {
        message?.contains("CLEARTEXT", ignoreCase = true) == true ->
            IOException("Android 阻止了明文 HTTP 连接，请检查应用网络策略或改用 HTTPS", this)
        message.isNullOrBlank() ->
            IOException("连接失败，请确认服务器地址、端口和局域网访问权限", this)
        else -> this
    }
    else -> IOException(message?.takeIf { it.isNotBlank() } ?: "连接失败，请确认服务器地址和网络状态", this)
}

class UnauthorizedException : IllegalStateException("登录状态已失效")
class NetworkException(message: String) : IOException(message)
class ServerException(val code: Int, message: String) : IOException(message)
