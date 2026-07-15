package com.nowen.video.v2.core.data

import androidx.paging.PagingSource
import androidx.paging.PagingState
import com.nowen.video.v2.core.model.LibraryFilter
import com.nowen.video.v2.core.model.MediaCard

internal class CatalogPagingSource(
    private val api: CatalogApi,
    private val filter: LibraryFilter,
) : PagingSource<Int, MediaCard>() {
    override suspend fun load(params: LoadParams<Int>): LoadResult<Int, MediaCard> {
        val page = params.key ?: 1
        val pageSize = params.loadSize.coerceIn(1, 200)
        return try {
            val response = api.media(
                page = page,
                size = pageSize,
                libraryId = filter.libraryId,
                contentType = filter.contentType.apiValue,
                genre = filter.genre.takeIf(String::isNotBlank),
                query = filter.query.takeIf(String::isNotBlank),
                yearFrom = filter.yearFrom,
                yearTo = filter.yearTo,
                sort = filter.sort.apiValue,
                order = filter.order.apiValue,
            )
            val items = response.data.filter { it.resolvedId.isNotBlank() }
            val reachedEnd = response.data.isEmpty() || page * pageSize >= response.total
            LoadResult.Page(
                data = items,
                prevKey = page.takeIf { it > 1 }?.minus(1),
                nextKey = if (reachedEnd) null else page + 1,
                itemsBefore = ((page - 1) * pageSize).coerceAtLeast(0),
                itemsAfter = (response.total - page * pageSize).coerceAtLeast(0),
            )
        } catch (error: Throwable) {
            LoadResult.Error(mapApiError(error))
        }
    }

    override fun getRefreshKey(state: PagingState<Int, MediaCard>): Int? =
        state.anchorPosition?.let { anchor ->
            state.closestPageToPosition(anchor)?.let { page ->
                page.prevKey?.plus(1) ?: page.nextKey?.minus(1)
            }
        }
}
