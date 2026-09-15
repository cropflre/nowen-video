package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.LibraryContentType
import com.nowen.video.v2.core.model.LibraryFilter
import com.nowen.video.v2.core.model.LibraryOrder
import com.nowen.video.v2.core.model.LibrarySort
import com.nowen.video.v2.core.model.MediaCard
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class LibraryLayoutTest {
    @Test
    fun compactAndExpandedWidthsUseStableBreakpoint() {
        assertFalse(isExpandedLibraryLayout(839))
        assertTrue(isExpandedLibraryLayout(840))
        assertTrue(isExpandedLibraryLayout(1280))
    }

    @Test
    fun summaryDescribesTypeSortAndFilters() {
        val summary = libraryFilterSummary(
            LibraryFilter(
                contentType = LibraryContentType.Series,
                genre = "科幻",
                yearFrom = 2020,
                yearTo = 2025,
                sort = LibrarySort.Rating,
                order = LibraryOrder.Descending,
            ),
        )

        assertEquals("剧集 · 评分降序 · 2020–2025 · 科幻", summary)
    }

    @Test
    fun genreShelvesUseOnlyRealGenresInStableOrder() {
        val shelves = landingGenreShelves(
            listOf(
                MediaCard(id = "one", genres = "动作 / 冒险"),
                MediaCard(id = "two", genres = "冒险，科幻"),
                MediaCard(id = "one", genres = "动作"),
                MediaCard(id = "untagged"),
            ),
        )

        assertEquals(listOf("动作", "冒险", "科幻"), shelves.map(LandingGenreShelf::genre))
        assertEquals(listOf("one"), shelves[0].items.map(MediaCard::resolvedId))
        assertEquals(listOf("one", "two"), shelves[1].items.map(MediaCard::resolvedId))
        assertEquals(listOf("two"), shelves[2].items.map(MediaCard::resolvedId))
    }
}
