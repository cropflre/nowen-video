package com.nowen.video.v2.core.data

import com.nowen.video.v2.core.model.ApiEnvelope
import com.nowen.video.v2.core.model.MediaCard
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class HomeContractTest {
    private val json = Json { ignoreUnknownKeys = true }

    @Test
    fun `normalizes manual server addresses`() {
        assertEquals("http://192.168.1.10:8080", UrlNormalizer.normalize("192.168.1.10:8080/"))
        assertEquals("https://media.example.com/base", UrlNormalizer.normalize("https://media.example.com/base/"))
        assertNull(UrlNormalizer.normalize("   "))
    }

    @Test
    fun `decodes continue watching records into media cards`() {
        val payload = """
            {
              "data": [{
                "media_id": "movie-1",
                "position": 50,
                "duration": 200,
                "media": {
                  "id": "movie-1",
                  "title": "流浪地球",
                  "poster_path": "/api/media/movie-1/poster",
                  "year": 2019
                }
              }]
            }
        """.trimIndent()

        val envelope = json.decodeFromString(
            ApiEnvelope.serializer(ListSerializer(MediaCard.serializer())),
            payload,
        )
        val media = envelope.data.single()
        assertEquals("movie-1", media.resolvedId)
        assertEquals("流浪地球", media.displayTitle)
        assertEquals("/api/media/movie-1/poster", media.resolvedPoster)
        assertEquals(0.25f, media.normalizedProgress, 0.0001f)
    }

    @Test
    fun `decodes mixed movie and series records`() {
        val payload = """
            [
              {"type":"movie","media":{"id":"m1","title":"电影","poster_path":"/m1"}},
              {"type":"series","series":{"id":"s1","title":"剧集","poster_path":"/s1"}}
            ]
        """.trimIndent()

        val items = json.decodeFromString(ListSerializer(MediaCard.serializer()), payload)
        assertEquals(listOf("m1", "s1"), items.map { it.resolvedId })
        assertEquals(listOf("movie", "series"), items.map { it.type })
        assertTrue(items.all { !it.resolvedPoster.isNullOrBlank() })
    }
}
