package session

import (
	"fmt"
	"path/filepath"
	"time"
)

const (
	defaultActiveTimeout      = 75 * time.Second
	defaultPausedTimeout      = 15 * time.Minute
	defaultSweepInterval      = 30 * time.Second
	defaultCloseDrainTimeout  = 5 * time.Second
	defaultAheadHighWatermark = 45 * time.Second
	defaultAheadLowWatermark  = 12 * time.Second
	defaultCleanupRetries     = 5
	defaultCleanupRetryDelay  = 200 * time.Millisecond
)

// Config defines the lifecycle and filesystem policy for temporary playback
// sessions. Runtime playback output is intentionally isolated from persistent
// transcode artifacts and is never recovered after a process restart.
type Config struct {
	RootDir string

	ActiveTimeout      time.Duration
	PausedTimeout      time.Duration
	SweepInterval      time.Duration
	CloseDrainTimeout  time.Duration
	AheadHighWatermark time.Duration
	AheadLowWatermark  time.Duration

	CleanupRetries    int
	CleanupRetryDelay time.Duration
}

// DefaultConfig derives the temporary playback root from the existing cache
// root without changing the legacy cache configuration contract.
func DefaultConfig(cacheRoot string) Config {
	return Config{
		RootDir:            filepath.Join(cacheRoot, "playback-temp"),
		ActiveTimeout:      defaultActiveTimeout,
		PausedTimeout:      defaultPausedTimeout,
		SweepInterval:      defaultSweepInterval,
		CloseDrainTimeout:  defaultCloseDrainTimeout,
		AheadHighWatermark: defaultAheadHighWatermark,
		AheadLowWatermark:  defaultAheadLowWatermark,
		CleanupRetries:     defaultCleanupRetries,
		CleanupRetryDelay:  defaultCleanupRetryDelay,
	}
}

func (c Config) normalized() (Config, error) {
	if c.RootDir == "" {
		return Config{}, fmt.Errorf("%w: playback session root directory is empty", ErrInvalidRequest)
	}
	root, err := filepath.Abs(filepath.Clean(c.RootDir))
	if err != nil {
		return Config{}, fmt.Errorf("resolve playback session root: %w", err)
	}
	c.RootDir = root

	if c.ActiveTimeout <= 0 {
		c.ActiveTimeout = defaultActiveTimeout
	}
	if c.PausedTimeout <= 0 {
		c.PausedTimeout = defaultPausedTimeout
	}
	if c.SweepInterval <= 0 {
		c.SweepInterval = defaultSweepInterval
	}
	if c.CloseDrainTimeout <= 0 {
		c.CloseDrainTimeout = defaultCloseDrainTimeout
	}
	if c.AheadHighWatermark <= 0 {
		c.AheadHighWatermark = defaultAheadHighWatermark
	}
	if c.AheadLowWatermark <= 0 {
		c.AheadLowWatermark = defaultAheadLowWatermark
	}
	if c.AheadLowWatermark >= c.AheadHighWatermark {
		return Config{}, fmt.Errorf("%w: low playback watermark must be lower than high watermark", ErrInvalidRequest)
	}
	if c.CleanupRetries <= 0 {
		c.CleanupRetries = defaultCleanupRetries
	}
	if c.CleanupRetryDelay <= 0 {
		c.CleanupRetryDelay = defaultCleanupRetryDelay
	}
	return c, nil
}
