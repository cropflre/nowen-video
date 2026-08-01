package service

// ParseStartupBridgeProfile identifies the virtual profile carried by the
// existing /stream/:id/:quality/:segment route.
func ParseStartupBridgeProfile(quality string) (string, bool) {
	return parseStartupVirtualProfile(quality)
}

// ParseStartupBridgeSegment identifies whether a virtual-profile segment comes
// from the immutable startup artifact or the live continuation artifact.
func ParseStartupBridgeSegment(segment string) (source string, actual string, ok bool) {
	return parseStartupBridgeSegment(segment)
}
