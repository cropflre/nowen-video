package com.nowen.video.v2.core.data

import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class SessionSecurityTest {
    @Test
    fun `authorization is limited to active server origin`() {
        val base = "https://video.example:8443"
        assertTrue(isSameOrigin(base, "https://video.example:8443/api/media/1".toHttpUrl()))
        assertFalse(isSameOrigin(base, "https://cdn.example/movie.mp4".toHttpUrl()))
        assertFalse(isSameOrigin(base, "http://video.example:8443/api/media/1".toHttpUrl()))
        assertFalse(isSameOrigin(base, "https://video.example/api/media/1".toHttpUrl()))
    }

    @Test
    fun `public auth endpoints never receive bearer token`() {
        val base = "https://video.example"
        assertFalse(shouldAttachSessionAuthorization(base, "https://video.example/api/auth/login".toHttpUrl()))
        assertFalse(shouldAttachSessionAuthorization(base, "https://video.example/api/auth/status".toHttpUrl()))
        assertFalse(shouldAttachSessionAuthorization(base, "https://video.example/api/auth/refresh".toHttpUrl()))
        assertTrue(shouldAttachSessionAuthorization(base, "https://video.example/api/libraries".toHttpUrl()))
    }

    @Test
    fun `session refreshes when expiry is unknown or near`() {
        assertTrue(shouldRefreshSession(0L, 1_000L))
        assertTrue(shouldRefreshSession(1_200L, 1_000L))
        assertFalse(shouldRefreshSession(1_301L, 1_000L))
    }
}
