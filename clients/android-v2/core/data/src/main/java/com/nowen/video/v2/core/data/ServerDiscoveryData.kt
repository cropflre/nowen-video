package com.nowen.video.v2.core.data

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.net.wifi.WifiManager
import android.util.Log
import com.nowen.video.v2.core.model.DiscoveredServer
import com.nowen.video.v2.core.model.DiscoverySource
import com.nowen.video.v2.core.model.InitStatusEnvelope
import com.nowen.video.v2.core.model.ServerQrPayload
import dagger.hilt.android.qualifiers.ApplicationContext
import java.net.Inet4Address
import java.net.NetworkInterface
import java.net.URI
import java.net.URLDecoder
import java.nio.charset.StandardCharsets
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.resume
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.channelFlow
import kotlinx.coroutines.flow.merge
import kotlinx.coroutines.joinAll
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withTimeout
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Request

private const val DISCOVERY_TIMEOUT_MS = 15_000L
private const val MDNS_SERVICE_TYPE = "_nowen-video._tcp."
private const val DISCOVERY_TAG = "NowenV2Discovery"

private data class ProbeEndpoint(val port: Int, val scheme: String)

private val DISCOVERY_ENDPOINTS = listOf(
    ProbeEndpoint(8080, "http"),
    ProbeEndpoint(80, "http"),
    ProbeEndpoint(3000, "http"),
    ProbeEndpoint(9090, "http"),
    ProbeEndpoint(8443, "https"),
    ProbeEndpoint(443, "https"),
)

data class ServerDiscoveryState(
    val isScanning: Boolean = false,
    val servers: List<DiscoveredServer> = emptyList(),
    val error: String? = null,
)

@Singleton
class ServerDiscoveryManager @Inject constructor(
    @ApplicationContext context: Context,
    client: OkHttpClient,
    json: Json,
) {
    private val mdns = MdnsServerDiscovery(context)
    private val sweep = HttpSweepServerDiscovery(context, client, json)
    private val parserJson = json
    private val _state = MutableStateFlow(ServerDiscoveryState())
    val state: StateFlow<ServerDiscoveryState> = _state.asStateFlow()
    private var discoveryJob: Job? = null

    fun startDiscovery(scope: CoroutineScope) {
        stopDiscovery()
        _state.value = ServerDiscoveryState(isScanning = true)
        discoveryJob = scope.launch {
            val discovered = linkedMapOf<String, DiscoveredServer>()
            try {
                withTimeout(DISCOVERY_TIMEOUT_MS) {
                    merge(
                        mdns.discover().catch { error ->
                            Log.w(DISCOVERY_TAG, "mDNS discovery failed", error)
                        },
                        sweep.discover().catch { error ->
                            Log.w(DISCOVERY_TAG, "HTTP sweep failed", error)
                        },
                    ).collect { server ->
                        val current = discovered[server.uniqueKey]
                        if (current == null ||
                            (server.source == DiscoverySource.MDNS && current.source != DiscoverySource.MDNS)
                        ) {
                            discovered[server.uniqueKey] = server
                        }
                        _state.value = _state.value.copy(
                            servers = discovered.values.sortedWith(
                                compareBy<DiscoveredServer> { it.source != DiscoverySource.MDNS }
                                    .thenBy { it.name.lowercase() }
                                    .thenBy { it.host },
                            ),
                            error = null,
                        )
                    }
                }
            } catch (_: TimeoutCancellationException) {
                Log.d(DISCOVERY_TAG, "LAN discovery finished with ${discovered.size} result(s)")
            } catch (cancelled: CancellationException) {
                throw cancelled
            } catch (error: Throwable) {
                _state.value = _state.value.copy(error = error.message ?: "局域网扫描失败")
            } finally {
                _state.value = _state.value.copy(isScanning = false)
            }
        }
    }

    fun stopDiscovery() {
        discoveryJob?.cancel()
        discoveryJob = null
        if (_state.value.isScanning) _state.value = _state.value.copy(isScanning = false)
    }

    fun parseQr(rawValue: String): ServerQrPayload? = parseServerQrPayload(rawValue, parserJson)
}

private class MdnsServerDiscovery(
    private val context: Context,
) {
    private val resolveMutex = Mutex()

    fun discover(): Flow<DiscoveredServer> = callbackFlow {
        val nsd = context.getSystemService(Context.NSD_SERVICE) as NsdManager
        val wifi = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as? WifiManager
        val multicastLock = wifi?.createMulticastLock("nowen-v2-mdns")?.apply {
            setReferenceCounted(false)
            runCatching { acquire() }
        }

        val listener = object : NsdManager.DiscoveryListener {
            override fun onDiscoveryStarted(serviceType: String) = Unit
            override fun onDiscoveryStopped(serviceType: String) = Unit
            override fun onServiceLost(serviceInfo: NsdServiceInfo) = Unit

            override fun onServiceFound(serviceInfo: NsdServiceInfo) {
                launch {
                    val resolved = resolveMutex.withLock { nsd.resolve(serviceInfo) }
                    resolved?.toDiscoveredServer()?.let { trySend(it) }
                }
            }

            override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {
                close(IllegalStateException("mDNS 启动失败：$errorCode"))
            }

            override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) = Unit
        }

        try {
            nsd.discoverServices(MDNS_SERVICE_TYPE, NsdManager.PROTOCOL_DNS_SD, listener)
        } catch (error: Throwable) {
            close(error)
        }

        awaitClose {
            runCatching { nsd.stopServiceDiscovery(listener) }
            runCatching { if (multicastLock?.isHeld == true) multicastLock.release() }
        }
    }

    private suspend fun NsdManager.resolve(serviceInfo: NsdServiceInfo): NsdServiceInfo? =
        suspendCancellableCoroutine { continuation ->
            val listener = object : NsdManager.ResolveListener {
                override fun onResolveFailed(info: NsdServiceInfo, errorCode: Int) {
                    if (continuation.isActive) continuation.resume(null)
                }

                override fun onServiceResolved(info: NsdServiceInfo) {
                    if (continuation.isActive) continuation.resume(info)
                }
            }
            try {
                @Suppress("DEPRECATION")
                resolveService(serviceInfo, listener)
            } catch (_: Throwable) {
                if (continuation.isActive) continuation.resume(null)
            }
        }

    @Suppress("DEPRECATION")
    private fun NsdServiceInfo.toDiscoveredServer(): DiscoveredServer? {
        val address = host?.hostAddress?.substringBefore('%')?.takeIf(String::isNotBlank) ?: return null
        val displayHost = if (address.contains(':')) "[$address]" else address
        val attributes = runCatching { attributes }.getOrDefault(emptyMap())
        val version = attributes["version"]?.toString(StandardCharsets.UTF_8).orEmpty()
        val serverName = attributes["server_name"]?.toString(StandardCharsets.UTF_8)
            ?.takeIf(String::isNotBlank)
            ?: serviceName
            .takeIf(String::isNotBlank)
            ?: "Nowen Video"
        return DiscoveredServer(
            name = serverName,
            host = address,
            port = port,
            version = version,
            source = DiscoverySource.MDNS,
            url = "http://$displayHost:$port",
        )
    }
}

private class HttpSweepServerDiscovery(
    private val context: Context,
    client: OkHttpClient,
    private val json: Json,
) {
    private val httpClient = client.newBuilder().apply {
        interceptors().clear()
        networkInterceptors().clear()
    }
        .connectTimeout(650, TimeUnit.MILLISECONDS)
        .readTimeout(1_200, TimeUnit.MILLISECONDS)
        .writeTimeout(1_200, TimeUnit.MILLISECONDS)
        .followRedirects(false)
        .followSslRedirects(false)
        .retryOnConnectionFailure(false)
        .build()

    fun discover(): Flow<DiscoveredServer> = channelFlow {
        val subnets = localPrivateSubnets(context)
        if (subnets.isEmpty()) return@channelFlow
        val semaphore = Semaphore(64)
        val jobs = buildList {
            subnets.forEach { subnet ->
                (1..254).forEach { suffix ->
                    val host = "$subnet.$suffix"
                    DISCOVERY_ENDPOINTS.forEach { endpoint ->
                        add(
                            launch(Dispatchers.IO) {
                                semaphore.withPermit {
                                    probe(host, endpoint)?.let { trySend(it) }
                                }
                            },
                        )
                    }
                }
            }
        }
        jobs.joinAll()
    }

    private fun probe(host: String, endpoint: ProbeEndpoint): DiscoveredServer? {
        val baseUrl = "${endpoint.scheme}://$host:${endpoint.port}"
        val request = Request.Builder()
            .url("$baseUrl/api/auth/status")
            .header("Accept", "application/json")
            .build()
        return runCatching {
            httpClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) return null
                val body = response.body?.string()?.takeIf(String::isNotBlank) ?: return null
                val status = json.decodeFromString<InitStatusEnvelope>(body).data
                DiscoveredServer(
                    name = status.serverName.ifBlank { "Nowen Video" },
                    host = host,
                    port = endpoint.port,
                    version = status.version,
                    source = DiscoverySource.HTTP_SWEEP,
                    url = baseUrl,
                )
            }
        }.getOrNull()
    }
}

internal fun parseServerQrPayload(rawValue: String, json: Json): ServerQrPayload? {
    val raw = rawValue.trim()
    if (raw.isBlank()) return null

    val candidate = when {
        raw.startsWith("{") -> parseJsonQrPayload(raw, json)
        raw.startsWith("nowen-video://", ignoreCase = true) ||
            raw.startsWith("nowen://", ignoreCase = true) -> parseCustomQrPayload(raw)
        else -> ServerQrPayload(url = raw)
    } ?: return null

    val normalized = UrlNormalizer.normalize(candidate.url) ?: return null
    val scheme = URI(normalized).scheme?.lowercase()
    if (scheme != "http" && scheme != "https") return null
    return candidate.copy(
        url = normalized,
        name = candidate.name.trim().take(80),
    )
}

private fun parseJsonQrPayload(raw: String, json: Json): ServerQrPayload? = runCatching {
    val objectValue = json.parseToJsonElement(raw) as? JsonObject ?: return null
    val url = objectValue.string("url")
        ?: objectValue.string("base_url")
        ?: objectValue.string("server_url")
        ?: return null
    ServerQrPayload(
        url = url,
        name = objectValue.string("name") ?: objectValue.string("server_name").orEmpty(),
    )
}.getOrNull()

private fun parseCustomQrPayload(raw: String): ServerQrPayload? = runCatching {
    val uri = URI(raw)
    if (uri.host?.lowercase() != "server" && uri.path?.trim('/')?.lowercase() != "server") return null
    val parameters = uri.rawQuery.orEmpty()
        .split('&')
        .mapNotNull { part ->
            val separator = part.indexOf('=')
            if (separator <= 0) return@mapNotNull null
            decodeQuery(part.substring(0, separator)) to decodeQuery(part.substring(separator + 1))
        }
        .toMap()
    val url = parameters["url"] ?: parameters["base_url"] ?: return null
    ServerQrPayload(url = url, name = parameters["name"].orEmpty())
}.getOrNull()

private fun decodeQuery(value: String): String =
    URLDecoder.decode(value, StandardCharsets.UTF_8.name())

private fun JsonObject.string(name: String): String? =
    this[name]?.jsonPrimitive?.content?.trim()?.takeIf(String::isNotBlank)

private fun localPrivateSubnets(context: Context): Set<String> {
    val subnets = linkedSetOf<String>()
    runCatching {
        val wifi = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as? WifiManager
        @Suppress("DEPRECATION")
        val ip = wifi?.dhcpInfo?.ipAddress ?: 0
        if (ip != 0) {
            val address = "${ip and 0xFF}.${ip shr 8 and 0xFF}.${ip shr 16 and 0xFF}.${ip shr 24 and 0xFF}"
            privateSubnet(address)?.let(subnets::add)
        }
    }
    runCatching {
        val interfaces = NetworkInterface.getNetworkInterfaces()
        while (interfaces.hasMoreElements()) {
            val network = interfaces.nextElement()
            if (!network.isUp || network.isLoopback) continue
            val addresses = network.inetAddresses
            while (addresses.hasMoreElements()) {
                val address = addresses.nextElement()
                if (address is Inet4Address && !address.isLoopbackAddress) {
                    privateSubnet(address.hostAddress.orEmpty())?.let(subnets::add)
                }
            }
        }
    }
    return subnets
}

internal fun privateSubnet(address: String): String? {
    val parts = address.split('.').mapNotNull(String::toIntOrNull)
    if (parts.size != 4 || parts.any { it !in 0..255 }) return null
    val privateAddress = parts[0] == 10 ||
        (parts[0] == 172 && parts[1] in 16..31) ||
        (parts[0] == 192 && parts[1] == 168)
    return if (privateAddress) "${parts[0]}.${parts[1]}.${parts[2]}" else null
}
