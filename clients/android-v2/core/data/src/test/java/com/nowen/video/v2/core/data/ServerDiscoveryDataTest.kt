package com.nowen.video.v2.core.data

import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ServerDiscoveryDataTest {
    private val json = Json { ignoreUnknownKeys = true }

    @Test
    fun `plain address QR is normalized`() {
        val payload = parseServerQrPayload("192.168.1.20:8080/", json)

        assertEquals("http://192.168.1.20:8080", payload?.url)
        assertEquals("", payload?.name)
    }

    @Test
    fun `JSON QR supports server metadata`() {
        val payload = parseServerQrPayload(
            """{"type":"nowen-video-server","server_name":"客厅 NAS","base_url":"https://video.example.com"}""",
            json,
        )

        assertEquals("https://video.example.com", payload?.url)
        assertEquals("客厅 NAS", payload?.name)
    }

    @Test
    fun `custom Nowen URI decodes URL and name`() {
        val payload = parseServerQrPayload(
            "nowen-video://server?url=http%3A%2F%2F192.168.50.9%3A8080&name=Home%20NAS",
            json,
        )

        assertEquals("http://192.168.50.9:8080", payload?.url)
        assertEquals("Home NAS", payload?.name)
    }

    @Test
    fun `unrelated QR text is rejected`() {
        assertNull(parseServerQrPayload("欢迎使用 Nowen Video", json))
        assertNull(parseServerQrPayload("nowen-video://profile?id=1", json))
    }

    @Test
    fun `only private IPv4 addresses produce sweep subnet`() {
        assertEquals("192.168.31", privateSubnet("192.168.31.42"))
        assertEquals("172.20.4", privateSubnet("172.20.4.8"))
        assertEquals("10.0.0", privateSubnet("10.0.0.2"))
        assertNull(privateSubnet("8.8.8.8"))
        assertNull(privateSubnet("not-an-ip"))
    }
}
