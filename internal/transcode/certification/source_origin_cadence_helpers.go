package certification

import transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"

func ticksToMicrosCertification(ticks int64, timeBase string) (int64, error) {
	return transcodeboundary.TicksToMicros(ticks, timeBase)
}
