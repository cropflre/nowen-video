package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.MediaPerson
import com.nowen.video.v2.core.model.Person
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class MediaDetailPersonsTest {
    @Test
    fun `uses episode specific persons when available`() {
        val direct = listOf(personCredit("episode-person"))
        val series = listOf(personCredit("series-person"))

        assertEquals(direct, selectDetailPersons(direct, series))
    }

    @Test
    fun `falls back to series persons when episode has none`() {
        val series = listOf(personCredit("series-person"))

        assertEquals(emptyList<MediaPerson>(), selectDetailPersons(emptyList(), emptyList()))
        assertEquals(series, selectDetailPersons(emptyList(), series))
    }

    @Test
    fun `keeps no persons empty when neither endpoint returns credits`() {
        assertTrue(selectDetailPersons(emptyList(), emptyList()).isEmpty())
    }

    private fun personCredit(id: String) = MediaPerson(
        id = "credit-$id",
        personId = id,
        person = Person(id = id, name = "Test $id"),
    )
}
