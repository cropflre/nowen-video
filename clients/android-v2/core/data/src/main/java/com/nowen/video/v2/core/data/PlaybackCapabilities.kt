package com.nowen.video.v2.core.data

import android.media.MediaCodecList
import com.nowen.video.v2.core.model.PlaybackClientCapabilities
import javax.inject.Inject
import javax.inject.Singleton

private const val HEVC_MIME_TYPE = "video/hevc"

@Singleton
class PlaybackCapabilityProvider @Inject constructor() {
    fun current(): PlaybackClientCapabilities = PlaybackClientCapabilities(
        supportsDirectPlay = true,
        supportsRemux = true,
        supportsHevc = detectHevcDecoder(),
    )
}

internal fun detectHevcDecoder(): Boolean = runCatching {
    MediaCodecList(MediaCodecList.ALL_CODECS).codecInfos.any { codec ->
        !codec.isEncoder && codecSupportsHevc(codec.supportedTypes)
    }
}.getOrDefault(false)

internal fun codecSupportsHevc(supportedTypes: Array<String>): Boolean =
    supportedTypes.any { it.equals(HEVC_MIME_TYPE, ignoreCase = true) }
