package com.nowen.video.v2.core.data

import androidx.paging.PagingSource
import com.nowen.video.v2.core.model.PaginatedEnvelope
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SocialPagingSourceTest {
    @Test
    fun `first page exposes next key while total remains`() = runBlocking {
        val source = SocialPagingSource<Int> { page, size ->
            PaginatedEnvelope(
                data = (1..size).toList(),
                total = 65,
                page = page,
                size = size,
            )
        }

        val result = source.load(PagingSource.LoadParams.Refresh(key = null, loadSize = 30, placeholdersEnabled = false))
            as PagingSource.LoadResult.Page

        assertNull(result.prevKey)
        assertEquals(2, result.nextKey)
        assertEquals(30, result.data.size)
    }

    @Test
    fun `last partial page terminates pagination`() = runBlocking {
        val source = SocialPagingSource<Int> { page, size ->
            PaginatedEnvelope(
                data = listOf(61, 62, 63, 64, 65),
                total = 65,
                page = page,
                size = size,
            )
        }

        val result = source.load(PagingSource.LoadParams.Append(key = 3, loadSize = 30, placeholdersEnabled = false))
            as PagingSource.LoadResult.Page

        assertEquals(2, result.prevKey)
        assertNull(result.nextKey)
        assertEquals(5, result.data.size)
    }
}
