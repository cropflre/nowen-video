package com.nowen.video.v2.core.data

import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicReference
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import okhttp3.Call
import okhttp3.EventListener
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ServerProbeDataTest {
    private lateinit var server: MockWebServer

    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
        isLenient = true
        explicitNulls = false
    }

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `health probe runs off caller thread and returns server metadata`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(
                    """
                    {
                      "status": "ok",
                      "server_name": "Living Room",
                      "version": "1.8.0",
                      "data": {
                        "server_name": "Living Room",
                        "version": "1.8.0"
                      }
                    }
                    """.trimIndent(),
                ),
        )

        val networkThread = AtomicReference<String>()
        val client = OkHttpClient.Builder()
            .eventListener(
                object : EventListener() {
                    override fun callStart(call: Call) {
                        networkThread.set(Thread.currentThread().name)
                    }
                },
            )
            .build()
        val executor = Executors.newSingleThreadExecutor { runnable -> Thread(runnable, "probe-caller") }
        executor.asCoroutineDispatcher().use { callerDispatcher ->
            val callerThread = AtomicReference<String>()
            val result = runBlocking(callerDispatcher) {
                callerThread.set(Thread.currentThread().name)
                probeServer(server.url("/").toString(), client, json)
            }

            assertTrue(result.isSuccess)
            assertEquals("Living Room", result.getOrThrow().serverName)
            assertEquals("1.8.0", result.getOrThrow().version)
            assertNotEquals(callerThread.get(), networkThread.get())
        }

        assertEquals("/api/health", server.takeRequest().path)
    }

    @Test
    fun `auth status is used when health endpoint is unavailable`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(404))
        server.enqueue(
            MockResponse()
                .setResponseCode(200)
                .setHeader("Content-Type", "application/json")
                .setBody(
                    """
                    {
                      "data": {
                        "initialized": true,
                        "server_name": "Fallback Server",
                        "version": "1.7.4"
                      }
                    }
                    """.trimIndent(),
                ),
        )

        val result = probeServer(server.url("/").toString(), OkHttpClient(), json)

        assertTrue(result.isSuccess)
        assertEquals("Fallback Server", result.getOrThrow().serverName)
        assertEquals("1.7.4", result.getOrThrow().version)
        assertEquals("/api/health", server.takeRequest().path)
        assertEquals("/api/auth/status", server.takeRequest().path)
    }

    @Test
    fun `probe failure keeps endpoint diagnostics`() = runBlocking {
        server.enqueue(MockResponse().setResponseCode(503))
        server.enqueue(MockResponse().setResponseCode(401))

        val result = probeServer(server.url("/").toString(), OkHttpClient(), json)

        assertTrue(result.isFailure)
        val message = result.exceptionOrNull()?.message.orEmpty()
        assertTrue(message.contains("api/health：HTTP 503"))
        assertTrue(message.contains("api/auth/status：HTTP 401"))
    }

    @Test
    fun `invalid address fails without sending a request`() = runBlocking {
        val result = probeServer("http://", OkHttpClient(), json)

        assertTrue(result.isFailure)
        assertEquals("服务器地址无效", result.exceptionOrNull()?.message)
        assertFalse(server.requestCount > 0)
    }
}
