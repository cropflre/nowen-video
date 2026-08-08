package service

// The static browser baseline must contain only codecs that are safe to assume
// without a client probe. FLAC and Opus support varies by browser/container and
// is now reported explicitly by the Web capability probe. Removing them from
// the unconditional baseline lets a positive client probe opt in, while a
// negative or missing probe stays conservative and can use Smart Remux.
func init() {
	delete(browserCompatibleAudioCodecs, "flac")
	delete(browserCompatibleAudioCodecs, "opus")
}
