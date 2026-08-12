package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MovieCollection
import com.nowen.video.v2.core.model.Person
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class SearchUiStateTest {
    @Test
    fun `results from any catalog section make state non-empty`() {
        assertTrue(SearchUiState(mediaResults = listOf(MediaCard(id = "media-1"))).hasResults)
        assertTrue(SearchUiState(peopleResults = listOf(Person(id = "person-1"))).hasResults)
        assertTrue(SearchUiState(collectionResults = listOf(MovieCollection(id = "collection-1"))).hasResults)
        assertFalse(SearchUiState().hasResults)
    }

    @Test
    fun `partial failures are exposed without replacing successful results`() {
        val state = SearchUiState(
            peopleResults = listOf(Person(id = "person-1", name = "演员")),
            unavailableSections = listOf("影视", "合集"),
        )

        assertTrue(state.hasResults)
        assertEquals("部分结果暂不可用：影视、合集", state.unavailableMessage)
        assertNull(SearchUiState().unavailableMessage)
    }
}
