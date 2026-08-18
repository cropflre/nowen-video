package com.nowen.video.v2.core.data

import com.nowen.video.v2.core.model.TokenResponse
import java.io.IOException
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

private const val SESSION_REFRESH_THRESHOLD_SECONDS = 5L * 60L

internal fun isSameOrigin(baseUrl: String?, targetUrl: HttpUrl): Boolean {
    val base = baseUrl?.let(UrlNormalizer::normalize)?.toHttpUrlOrNull() ?: return false
    return base.scheme.equals(targetUrl.scheme, ignoreCase = true) &&
        base.host.equals(targetUrl.host, ignoreCase = true) &&
        base.port == targetUrl.port
}

internal fun shouldRefreshSession(
    expiresAtEpochSeconds: Long,
    nowEpochSeconds: Long,
    thresholdSeconds: Long = SESSION_REFRESH_THRESHOLD_SECONDS,
): Boolean = expiresAtEpochSeconds <= 0L || expiresAtEpochSeconds - nowEpochSeconds <= thresholdSeconds

internal fun isPublicSessionEndpoint(url: HttpUrl): Boolean {
    val path = url.encodedPath.trimEnd('/')
    return path.endsWith("/api/auth/status") ||
        path.endsWith("/api/auth/login") ||
        path.endsWith("/api/auth/register") ||
        path.endsWith("/api/auth/refresh")
}

internal fun shouldAttachSessionAuthorization(baseUrl: String?, targetUrl: HttpUrl): Boolean =
    isSameOrigin(baseUrl, targetUrl) && !isPublicSessionEndpoint(targetUrl)

internal fun shouldAuthorizeDownload(serverBaseUrl: String?, sourceUrl: String?): Boolean {
    val target = sourceUrl?.toHttpUrlOrNull() ?: return false
    return isSameOrigin(serverBaseUrl, target)
}

@Singleton
class SessionRefreshCoordinator @Inject constructor(
    private val sessionStore: ServerSessionStore,
    private val json: Json,
) {
    private val refreshMutex = Mutex()
    private val directClient = OkHttpClient.Builder()
        .connectTimeout(12, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .writeTimeout(20, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .build()

    suspend fun refreshIfNeeded(
        force: Boolean = false,
        failedToken: String? = null,
    ): String? = refreshMutex.withLock {
        val snapshot = sessionStore.snapshot.value
        val server = snapshot.activeServer ?: return@withLock null
        val currentToken = snapshot.token?.takeIf(String::isNotBlank) ?: return@withLock null

        if (force && !failedToken.isNullOrBlank() && failedToken != currentToken) {
            return@withLock currentToken
        }

        val now = currentEpochSeconds()
        val expiresAt = sessionStore.expiresAtEpochSeconds(server.id)
        if (expiresAt > 0L && expiresAt <= now) {
            sessionStore.clearAuthentication()
            return@withLock null
        }
        if (!force && !shouldRefreshSession(expiresAt, now)) {
            return@withLock currentToken
        }

        refresh(server.baseUrl, currentToken)
    }

    private suspend fun refresh(baseUrl: String, currentToken: String): String? {
        val refreshUrl = UrlNormalizer.apiUrl(baseUrl, "api/auth/refresh")?.toHttpUrlOrNull()
            ?: return currentToken
        if (!isSameOrigin(baseUrl, refreshUrl)) return currentToken

        return try {
            withContext(Dispatchers.IO) {
                val request = Request.Builder()
                    .url(refreshUrl)
                    .post(ByteArray(0).toRequestBody(null))
                    .header("Accept", "application/json")
                    .header("Authorization", "Bearer $currentToken")
                    .build()
                directClient.newCall(request).execute().use { response ->
                    when {
                        response.code == 401 || response.code == 403 -> {
                            sessionStore.clearAuthentication()
                            null
                        }
                        !response.isSuccessful -> currentToken
                        else -> {
                            val body = response.body?.string().orEmpty()
                            if (body.isBlank()) return@use currentToken
                            val refreshed = json.decodeFromString<TokenResponse>(body)
                            val user = refreshed.user.copy(
                                mustChangePassword = refreshed.user.mustChangePassword || refreshed.mustChangePassword,
                            )
                            sessionStore.saveAuthenticatedSession(
                                refreshed.token,
                                user,
                                refreshed.expiresAt,
                            )
                            refreshed.token
                        }
                    }
                }
            }
        } catch (_: IOException) {
            currentToken
        } catch (_: IllegalArgumentException) {
            currentToken
        }
    }
}

class SessionInterceptor(
    private val sessionStore: ServerSessionStore,
    private val refreshCoordinator: SessionRefreshCoordinator,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val original = chain.request()
        val initialSnapshot = sessionStore.snapshot.value
        val server = initialSnapshot.activeServer
        val targetUrl = if (server != null && original.url.host == PLACEHOLDER_HOST) {
            val base = server.baseUrl.toHttpUrlOrNull()
            base?.newBuilder()
                ?.addPathSegments(original.url.encodedPath.trimStart('/'))
                ?.encodedQuery(original.url.encodedQuery)
                ?.build()
                ?: original.url
        } else {
            original.url
        }

        val attachAuthorization = server != null && shouldAttachSessionAuthorization(server.baseUrl, targetUrl)
        val token = if (attachAuthorization && !initialSnapshot.token.isNullOrBlank()) {
            runBlocking { refreshCoordinator.refreshIfNeeded() }
        } else {
            null
        }

        val request = original.newBuilder()
            .url(targetUrl)
            .header("Accept", "application/json")
            .apply {
                removeHeader("Authorization")
                token?.takeIf(String::isNotBlank)?.let { header("Authorization", "Bearer $it") }
            }
            .build()

        val response = chain.proceed(request)
        if (response.code != 401 || !attachAuthorization || token.isNullOrBlank()) {
            return response
        }

        val refreshedToken = runBlocking {
            refreshCoordinator.refreshIfNeeded(force = true, failedToken = token)
        }
        if (refreshedToken.isNullOrBlank() || refreshedToken == token) {
            return response
        }

        response.close()
        val retry = request.newBuilder()
            .header("Authorization", "Bearer $refreshedToken")
            .build()
        return chain.proceed(retry)
    }
}
