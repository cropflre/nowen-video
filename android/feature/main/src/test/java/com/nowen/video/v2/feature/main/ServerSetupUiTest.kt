package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.DiscoverySource
import org.junit.Assert.assertEquals
import org.junit.Test

class ServerSetupUiTest {
    @Test
    fun `discovery sources have user facing labels`() {
        assertEquals("mDNS 自动发现", discoverySourceLabel(DiscoverySource.MDNS))
        assertEquals("局域网端口探测", discoverySourceLabel(DiscoverySource.HTTP_SWEEP))
        assertEquals("二维码", discoverySourceLabel(DiscoverySource.QR))
    }

    @Test
    fun `server keys ignore trailing slash and case`() {
        assertEquals(
            normalizedServerKey("HTTP://192.168.1.10:8080/"),
            normalizedServerKey("http://192.168.1.10:8080"),
        )
    }
}
