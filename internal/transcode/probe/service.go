package probe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

var ErrUnsupportedSource = errors.New("media source cannot be probed directly")

const defaultProbeTimeout = 20 * time.Second

type Stats struct {
	CacheHits   uint64 `json:"cache_hits"`
	CacheMisses uint64 `json:"cache_misses"`
	Executions  uint64 `json:"executions"`
	Failures    uint64 `json:"failures"`
}

type Service struct {
	repo        *repository.MediaProbeRepo
	ffprobePath string
	logger      *zap.SugaredLogger
	timeout     time.Duration
	group       singleflight.Group

	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64
	executions  atomic.Uint64
	failures    atomic.Uint64
}

type sourceIdentity struct {
	MediaID     string
	Path        string
	Size        int64
	ModTimeNS   int64
	Fingerprint string
}

func NewService(db *gorm.DB, ffprobePath string, logger *zap.SugaredLogger) (*Service, error) {
	if db == nil {
		return nil, fmt.Errorf("media probe database is required")
	}
	repo := repository.NewMediaProbeRepo(db)
	if err := repo.AutoMigrate(); err != nil {
		return nil, fmt.Errorf("migrate media probe cache: %w", err)
	}
	if strings.TrimSpace(ffprobePath) == "" {
		ffprobePath = "ffprobe"
	}
	return &Service{
		repo:        repo,
		ffprobePath: ffprobePath,
		logger:      logger,
		timeout:     defaultProbeTimeout,
	}, nil
}

func (s *Service) Stats() Stats {
	if s == nil {
		return Stats{}
	}
	return Stats{
		CacheHits:   s.cacheHits.Load(),
		CacheMisses: s.cacheMisses.Load(),
		Executions:  s.executions.Load(),
		Failures:    s.failures.Load(),
	}
}

// Probe returns a fresh persisted technical description. The shared FFprobe
// execution uses an independent timeout so one disconnected client does not
// cancel work needed by other concurrent playback requests.
func (s *Service) Probe(ctx context.Context, media *model.Media) (*model.MediaProbeRecord, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("media probe service is unavailable")
	}
	identity, err := identifySource(media)
	if err != nil {
		return nil, err
	}
	if cached, cacheErr := s.repo.FindFresh(identity.MediaID, identity.Fingerprint, model.MediaProbeVersion); cacheErr == nil {
		s.cacheHits.Add(1)
		return cached, nil
	} else if !errors.Is(cacheErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("read media probe cache: %w", cacheErr)
	}
	s.cacheMisses.Add(1)

	key := identity.MediaID + "|" + identity.Fingerprint
	resultCh := s.group.DoChan(key, func() (any, error) {
		// Another caller may have populated the row while this caller waited for
		// the single-flight slot.
		if cached, cacheErr := s.repo.FindFresh(identity.MediaID, identity.Fingerprint, model.MediaProbeVersion); cacheErr == nil {
			return cached, nil
		} else if !errors.Is(cacheErr, gorm.ErrRecordNotFound) {
			return nil, cacheErr
		}

		probeCtx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		record, probeErr := s.execute(probeCtx, identity)
		if probeErr != nil {
			s.failures.Add(1)
			return nil, probeErr
		}
		if persistErr := s.repo.Upsert(record); persistErr != nil {
			s.failures.Add(1)
			return nil, fmt.Errorf("persist media probe: %w", persistErr)
		}
		return record, nil
	})

	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		record, ok := result.Val.(*model.MediaProbeRecord)
		if !ok || record == nil {
			return nil, fmt.Errorf("invalid media probe result")
		}
		return record, nil
	}
}

func (s *Service) execute(ctx context.Context, identity sourceIdentity) (*model.MediaProbeRecord, error) {
	s.executions.Add(1)
	cmd := exec.CommandContext(ctx, s.ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		identity.Path,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ffprobe timeout or cancellation: %w", ctx.Err())
		}
		message := strings.TrimSpace(string(output))
		if len(message) > 2048 {
			message = message[len(message)-2048:]
		}
		return nil, fmt.Errorf("ffprobe failed: %w: %s", err, message)
	}

	record, err := parseFFprobeOutput(output)
	if err != nil {
		return nil, err
	}
	record.MediaID = identity.MediaID
	record.SourceFingerprint = identity.Fingerprint
	record.SourcePath = identity.Path
	record.SourceSize = identity.Size
	record.SourceModTimeNS = identity.ModTimeNS
	record.ProbeVersion = model.MediaProbeVersion
	record.ProbedAt = time.Now()
	return record, nil
}

func identifySource(media *model.Media) (sourceIdentity, error) {
	if media == nil || strings.TrimSpace(media.ID) == "" {
		return sourceIdentity{}, fmt.Errorf("media id is required")
	}
	path := strings.TrimSpace(media.FilePath)
	if path == "" {
		return sourceIdentity{}, fmt.Errorf("media source path is required")
	}
	lowerPath := strings.ToLower(path)
	if strings.HasSuffix(lowerPath, ".strm") || strings.HasPrefix(lowerPath, "webdav://") {
		return sourceIdentity{}, ErrUnsupportedSource
	}

	size := media.FileSize
	var modTimeNS int64
	if !strings.Contains(path, "://") {
		info, err := os.Stat(path)
		if err != nil {
			return sourceIdentity{}, fmt.Errorf("stat media source: %w", err)
		}
		if info.IsDir() {
			return sourceIdentity{}, fmt.Errorf("media source is a directory")
		}
		size = info.Size()
		modTimeNS = info.ModTime().UnixNano()
	} else if !media.UpdatedAt.IsZero() {
		// Remote HTTP sources have no local mtime. Media UpdatedAt keeps the
		// cache invalidatable when the remote URL or metadata is refreshed.
		modTimeNS = media.UpdatedAt.UnixNano()
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf(
		"%s|%d|%d|%s",
		path,
		size,
		modTimeNS,
		model.MediaProbeVersion,
	)))
	return sourceIdentity{
		MediaID:     media.ID,
		Path:        path,
		Size:        size,
		ModTimeNS:   modTimeNS,
		Fingerprint: hex.EncodeToString(hash[:]),
	}, nil
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
}

type ffprobeStream struct {
	Index            int               `json:"index"`
	CodecType        string            `json:"codec_type"`
	CodecName        string            `json:"codec_name"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	PixelFormat      string            `json:"pix_fmt"`
	AverageFrameRate string            `json:"avg_frame_rate"`
	RealFrameRate    string            `json:"r_frame_rate"`
	Duration         string            `json:"duration"`
	BitsPerRawSample string            `json:"bits_per_raw_sample"`
	ColorTransfer    string            `json:"color_transfer"`
	ColorPrimaries   string            `json:"color_primaries"`
	ColorSpace       string            `json:"color_space"`
	ColorRange       string            `json:"color_range"`
	Channels         int               `json:"channels"`
	ChannelLayout    string            `json:"channel_layout"`
	SampleRate       string            `json:"sample_rate"`
	Tags             map[string]string `json:"tags"`
	Disposition      struct {
		Default int `json:"default"`
	} `json:"disposition"`
	SideDataList []struct {
		SideDataType string `json:"side_data_type"`
	} `json:"side_data_list"`
}

func parseFFprobeOutput(data []byte) (*model.MediaProbeRecord, error) {
	var output ffprobeOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("decode ffprobe json: %w", err)
	}

	record := &model.MediaProbeRecord{FormatName: output.Format.FormatName}
	if duration, err := strconv.ParseFloat(output.Format.Duration, 64); err == nil && duration > 0 {
		record.DurationMS = int64(math.Round(duration * 1000))
	}

	var audioStreams []model.MediaProbeAudioStream
	videoFound := false
	for _, stream := range output.Streams {
		switch stream.CodecType {
		case "video":
			if videoFound {
				continue
			}
			videoFound = true
			record.VideoCodec = strings.ToLower(stream.CodecName)
			record.Width = stream.Width
			record.Height = stream.Height
			record.PixelFormat = strings.ToLower(stream.PixelFormat)
			record.BitDepth = detectBitDepth(stream.BitsPerRawSample, stream.PixelFormat)
			record.ColorTransfer = strings.ToLower(stream.ColorTransfer)
			record.ColorPrimaries = strings.ToLower(stream.ColorPrimaries)
			record.ColorSpace = strings.ToLower(stream.ColorSpace)
			record.ColorRange = strings.ToLower(stream.ColorRange)
			record.FrameRateNum, record.FrameRateDen = parseFrameRate(stream.AverageFrameRate)
			if record.FrameRateNum == 0 {
				record.FrameRateNum, record.FrameRateDen = parseFrameRate(stream.RealFrameRate)
			}
			if record.DurationMS == 0 {
				if duration, err := strconv.ParseFloat(stream.Duration, 64); err == nil && duration > 0 {
					record.DurationMS = int64(math.Round(duration * 1000))
				}
			}
			record.HDR = detectHDR(stream)
		case "audio":
			sampleRate, _ := strconv.Atoi(stream.SampleRate)
			audioStreams = append(audioStreams, model.MediaProbeAudioStream{
				Index:         stream.Index,
				Codec:         strings.ToLower(stream.CodecName),
				Channels:      stream.Channels,
				ChannelLayout: stream.ChannelLayout,
				SampleRate:    sampleRate,
				Language:      stream.Tags["language"],
				Title:         stream.Tags["title"],
				Default:       stream.Disposition.Default == 1,
			})
		}
	}
	if !videoFound {
		return nil, fmt.Errorf("ffprobe output contains no video stream")
	}
	if err := record.SetAudioStreams(audioStreams); err != nil {
		return nil, fmt.Errorf("encode audio probe streams: %w", err)
	}
	return record, nil
}

func parseFrameRate(value string) (int, int) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 {
		return 0, 0
	}
	numerator, errNum := strconv.Atoi(parts[0])
	denominator, errDen := strconv.Atoi(parts[1])
	if errNum != nil || errDen != nil || numerator <= 0 || denominator <= 0 {
		return 0, 0
	}
	divisor := gcd(numerator, denominator)
	return numerator / divisor, denominator / divisor
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	if a == 0 {
		return 1
	}
	return a
}

var pixelDepthPattern = regexp.MustCompile(`p(\d{2})(?:le|be)?$`)

func detectBitDepth(raw, pixelFormat string) int {
	if value, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && value > 0 {
		return value
	}
	format := strings.ToLower(pixelFormat)
	if strings.Contains(format, "p010") {
		return 10
	}
	if strings.Contains(format, "p012") {
		return 12
	}
	if match := pixelDepthPattern.FindStringSubmatch(format); len(match) == 2 {
		if value, err := strconv.Atoi(match[1]); err == nil && value >= 8 && value <= 16 {
			return value
		}
	}
	if format != "" {
		return 8
	}
	return 0
}

func detectHDR(stream ffprobeStream) bool {
	switch strings.ToLower(stream.ColorTransfer) {
	case "smpte2084", "smpte-st-2084", "arib-std-b67":
		return true
	}
	for _, sideData := range stream.SideDataList {
		kind := strings.ToLower(sideData.SideDataType)
		if strings.Contains(kind, "mastering display") ||
			strings.Contains(kind, "content light") ||
			strings.Contains(kind, "dovi") ||
			strings.Contains(kind, "dolby vision") {
			return true
		}
	}
	return false
}

func ApplyToMedia(media *model.Media, record *model.MediaProbeRecord) {
	if media == nil || record == nil {
		return
	}
	media.VideoCodec = record.VideoCodec
	if streams := record.AudioStreams(); len(streams) > 0 {
		media.AudioCodec = streams[0].Codec
	}
	if record.DurationMS > 0 {
		media.Duration = float64(record.DurationMS) / 1000
	}
	if record.Width > 0 && record.Height > 0 {
		media.Resolution = classifyResolution(record.Width, record.Height)
	}
}

func classifyResolution(width, height int) string {
	shortEdge := height
	if width < height {
		shortEdge = width
	}
	switch {
	case shortEdge >= 2160:
		return "4K"
	case shortEdge >= 1440:
		return "2K"
	case shortEdge >= 1080:
		return "1080p"
	case shortEdge >= 720:
		return "720p"
	case shortEdge >= 480:
		return "480p"
	case shortEdge > 0:
		return fmt.Sprintf("%dp", shortEdge)
	default:
		return ""
	}
}

func (s *Service) logDebugf(template string, args ...any) {
	if s != nil && s.logger != nil {
		s.logger.Debugf(template, args...)
	}
}

func sourceBase(path string) string {
	if strings.Contains(path, "://") {
		return path
	}
	return filepath.Clean(path)
}
