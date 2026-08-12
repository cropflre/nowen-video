package com.nowen.video.v2.feature.main

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SocialCatalogUiTest {
    @Test
    fun `collection poster uses authenticated server origin`() {
        assertEquals(
            "http://192.168.1.10:8080/api/collections/coll-1/poster",
            collectionPosterUrl("http://192.168.1.10:8080/", "coll-1"),
        )
        assertNull(collectionPosterUrl(null, "coll-1"))
    }

    @Test
    fun `person profile uses server image endpoint`() {
        assertEquals(
            "https://video.example.com/api/persons/person-1/profile",
            personProfileUrl("https://video.example.com", "person-1"),
        )
        assertNull(personProfileUrl(null, "person-1"))
    }
}
