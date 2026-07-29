package com.nowen.video.v2.core.data

import java.io.IOException
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ServerHandshakeTest {
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    @Test
    fun `handshake always uses public auth status endpoint`() {
        val request = buildServerHandshakeRequest("192.168.1.115:8080")

        assertEquals("http", request.url.scheme)
        assertEquals("192.168.1.115", request.url.host)
        assertEquals(8080, request.url.port)
        assertEquals("/api/auth/status", request.url.encodedPath)
    }

    @Test
    fun `handshake preserves reverse proxy base path`() {
        val request = buildServerHandshakeRequest("https://video.example.com/nowen")

        assertEquals("/nowen/api/auth/status", request.url.encodedPath)
    }

    @Test
    fun `valid auth status returns server metadata`() {
        val handshake = decodeServerHandshake(
            """{"data":{"initialized":true,"server_name":"客厅媒体库","version":"v0.1.0"}}""",
            json,
        )

        assertEquals("客厅媒体库", handshake.serverName)
        assertEquals("v0.1.0", handshake.version)
    }

    @Test
    fun `unrelated successful response is rejected`() {
        val error = runCatching {
            decodeServerHandshake("""{"status":"ok"}""", json)
        }.exceptionOrNull()

        assertTrue(error is IOException)
        assertTrue(error?.message.orEmpty().contains("Nowen Video"))
    }

    @Test
    fun `blank response is rejected with actionable message`() {
        val error = runCatching {
            decodeServerHandshake("   ", json)
        }.exceptionOrNull()

        assertTrue(error is IOException)
        assertTrue(error?.message.orEmpty().contains("响应为空"))
    }
}
