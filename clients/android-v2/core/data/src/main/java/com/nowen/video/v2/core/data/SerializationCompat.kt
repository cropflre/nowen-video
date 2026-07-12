package com.nowen.video.v2.core.data

import kotlinx.serialization.json.Json
import kotlinx.serialization.serializer

inline fun <reified T> Json.decodeFromString(value: String): T =
    decodeFromString(serializer(), value)
