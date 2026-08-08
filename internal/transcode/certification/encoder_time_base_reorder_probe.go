package certification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

func probeEncoderTimeBasePacketOrder(
	ctx context.Context,
	ffprobePath,
	path,
	kind string,
) (transcodereorder.PacketOrderEvidence, error) {
	command := exec.CommandContext(
		ctx,
		ffprobePath,
		"-v", "error",
		"-i", path,
		"-print_format", "json",
		"-show_streams",
		"-show_packets",
		"-show_entries", "stream=index,codec_type,time_base:packet=stream_index,pts,dts",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return transcodereorder.PacketOrderEvidence{}, fmt.Errorf("ffprobe reorder failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var document sourceOriginProbeDocument
	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&document); err != nil {
		return transcodereorder.PacketOrderEvidence{}, fmt.Errorf("decode reorder probe: %w", err)
	}
	stream, ok := findSourceOriginStream(document.Streams, transcodesourceorigin.StreamVideo)
	if !ok {
		return transcodereorder.PacketOrderEvidence{}, fmt.Errorf("reorder probe has no video stream")
	}
	packets := make([]transcodereorder.PacketTimestamp, 0, len(document.Packets))
	for _, packet := range document.Packets {
		if packet.StreamIndex != stream.Index {
			continue
		}
		pts, ptsOK := packet.PTS.int64Value()
		dts, dtsOK := packet.DTS.int64Value()
		if !ptsOK || !dtsOK {
			return transcodereorder.PacketOrderEvidence{}, fmt.Errorf("video packet PTS/DTS is unavailable")
		}
		packets = append(packets, transcodereorder.PacketTimestamp{PTS: pts, DTS: dts})
	}
	evidence, err := transcodereorder.NewPacketOrderEvidence(kind, stream.TimeBase, packets)
	if err != nil {
		return transcodereorder.PacketOrderEvidence{}, err
	}
	return evidence, nil
}
