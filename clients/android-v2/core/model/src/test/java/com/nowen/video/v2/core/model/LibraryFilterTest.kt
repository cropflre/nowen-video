package com.nowen.video.v2.core.model

import org.junit.Assert.assertEquals
import org.junit.Test

class LibraryFilterTest {
    @Test
    fun normalizedTrimsValuesAndOrdersYearRange() {
        val normalized = LibraryFilter(
            libraryId = " library-1 ",
            genre = " 科幻 ",
            query = " 星球 ",
            yearFrom = 2025,
            yearTo = 1999,
        ).normalized()

        assertEquals("library-1", normalized.libraryId)
        assertEquals("科幻", normalized.genre)
        assertEquals("星球", normalized.query)
        assertEquals(1999, normalized.yearFrom)
        assertEquals(2025, normalized.yearTo)
    }

    @Test
    fun normalizedDropsInvalidYearsAndBlankLibrary() {
        val normalized = LibraryFilter(
            libraryId = "   ",
            yearFrom = 1200,
            yearTo = 4000,
        ).normalized()

        assertEquals(null, normalized.libraryId)
        assertEquals(null, normalized.yearFrom)
        assertEquals(null, normalized.yearTo)
    }

    @Test
    fun activeFilterCountExcludesSortingPreferences() {
        val filter = LibraryFilter(
            libraryId = "library-1",
            contentType = LibraryContentType.Movies,
            genre = "动作",
            query = "hero",
            yearFrom = 2020,
            sort = LibrarySort.Rating,
            order = LibraryOrder.Ascending,
        )

        assertEquals(5, filter.activeFilterCount)
    }
}
