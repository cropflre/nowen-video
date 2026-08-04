package emby

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	playbacksession "github.com/nowen-video/nowen-video/internal/playback/session"
	"github.com/nowen-video/nowen-video/internal/service"
)

const (
	embyPlaybackReadyTimeout   = 15 * time.Second
	embyPlaybackPollInterval   = 250 * time.Millisecond
	embyPlaybackSeekBack       = 5 * time.Second
	embyPlaybackSeekForward    = 45 * time.Second
	embyPlaybackBufferedWindow = 15 * time.Second
)

var (
	defaultPlaybackSessionMu sync.RWMutex
	defaultPlaybackSessions *service.PlaybackSessionService
	embyPlaybackRuntimes     sync.Map // map[*Handler]*embyPlaybackRuntime
)

// SetDefaultPlaybackSessionService connects the Full server's single playback
// runtime to the Emby compatibility layer. Lite does not mount Emby routes.
func SetDefaultPlaybackSessionService(sessions *service.PlaybackSessionService) {
	defaultPlaybackSessionMu.Lock()
	defaultPlaybackSessions = sessions
	defaultPlaybackSessionMu.Unlock()
}

func currentPlaybackSessionService() *service.PlaybackSessionService {
	defaultPlaybackSessionMu.RLock()
	defer defaultPlaybackSessionMu.RUnlock()
	return defaultPlaybackSessions
}

type embyPlaybackMapping struct {
	ExternalID    string
	InternalID    string
	UserID        string
	MediaID       string
	GenerationID  uint64
	GenerationPos int64
	ProfileID     string
	MaxBitrate    int
	LastPosition  int64
	UpdatedAt     time.Time
}

type embyPlaybackRuntime struct {
	sessions *service.PlaybackSessionService
	mu       sync.RWMutex
	entries  map[string]*embyPlaybackMapping
	stripes  [32]sync.Mutex
}

func (h *Handler) playbackSessionRuntime() *embyPlaybackRuntime {
	if h == nil {
		return nil
	}
	if value, ok := embyPlaybackRuntimes.Load(h); ok {
		return value.(*embyPlaybackRuntime)
	}
	sessions := currentPlaybackSessionService()
	if sessions == nil {
		return nil
	}
	runtime := &embyPlaybackRuntime{
		sessions: sessions,
		entries:  make(map[string]*embyPlaybackMapping),
	}
	actual, _ := embyPlaybackRuntimes.LoadOrStore(h, runtime)
	return actual.(*embyPlaybackRuntime)
}

func embyPlaybackKey(userID, externalID string) string {
	return strings.TrimSpace(userID) + "|" + strings.TrimSpace(externalID)
}

func (r *embyPlaybackRuntime) lockKey(key string) func() {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	lock := &r.stripes[int(hasher.Sum32())%len(r.stripes)]
	lock.Lock()
	return lock.Unlock
}

func (r *embyPlaybackRuntime) get(userID, externalID string) (*embyPlaybackMapping, bool) {
	if r == nil || externalID == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	mapping, ok := r.entries[embyPlaybackKey(userID, externalID)]
	if !ok {
		return nil, false
	}
	copy := *mapping
	return &copy, true
}

func (r *embyPlaybackRuntime) find(userID, externalID, mediaID string) (*embyPlaybackMapping, bool) {
	if mapping, ok := r.get(userID, externalID); ok {
		return mapping, true
	}
	if r == nil || mediaID == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var newest *embyPlaybackMapping
	for _, mapping := range r.entries {
		if mapping.UserID != userID || mapping.MediaID != mediaID {
			continue
		}
		if newest == nil || mapping.UpdatedAt.After(newest.UpdatedAt) {
			copy := *mapping
			newest = &copy
		}
	}
	return newest, newest != nil
}

func (r *embyPlaybackRuntime) put(mapping *embyPlaybackMapping) {
	if r == nil || mapping == nil {
		return
	}
	copy := *mapping
	copy.UpdatedAt = time.Now()
	r.mu.Lock()
	r.entries[embyPlaybackKey(copy.UserID, copy.ExternalID)] = &copy
	r.mu.Unlock()
}

func (r *embyPlaybackRuntime) delete(userID, externalID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.entries, embyPlaybackKey(userID, externalID))
	r.mu.Unlock()
}

func (r *embyPlaybackRuntime) ensure(
	ctx context.Context,
	userID,
	mediaID,
	externalID string,
	startPositionMS int64,
	hasStartPosition bool,
	maxBitrate int,
) (*embyPlaybackMapping, error) {
	if r == nil || r.sessions == nil {
		return nil, errors.New("playback session runtime is unavailable")
	}
	if externalID == "" {
		externalID = newSessionID(userID, mediaID)
	}
	key := embyPlaybackKey(userID, externalID)
	unlock := r.lockKey(key)
	defer unlock()

	if existing, ok := r.get(userID, externalID); ok {
		if existing.MediaID != mediaID {
			r.closeLocked(ctx, existing, "emby_session_reused")
			r.delete(userID, externalID)
		} else if _, err := r.sessions.Status(userID, existing.InternalID); err == nil {
			if hasStartPosition && absInt64(startPositionMS-existing.GenerationPos) > 1500 {
				return r.restartLocked(ctx, existing, startPositionMS, "emby_master_seek")
			}
			return existing, nil
		} else {
			r.delete(userID, externalID)
		}
	}

	result, err := r.sessions.Create(ctx, userID, service.PlaybackSessionCreateRequest{
		MediaID:         mediaID,
		ProfileID:       "auto",
		StartPositionMS: startPositionMS,
		SubtitleTrack:   -1,
		MaxBitrate:      maxBitrate,
	})
	if err != nil {
		return nil, err
	}
	ready, err := r.waitReady(ctx, userID, result)
	if err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = r.sessions.Close(closeCtx, userID, result.Session.ID, "emby_start_failed")
		cancel()
		return nil, err
	}
	mapping := &embyPlaybackMapping{
		ExternalID:    externalID,
		InternalID:    ready.Session.ID,
		UserID:        userID,
		MediaID:       mediaID,
		GenerationID:  ready.Session.CurrentGenerationID,
		GenerationPos: startPositionMS,
		ProfileID:     generationProfile(ready, "auto"),
		MaxBitrate:    maxBitrate,
		LastPosition:  startPositionMS,
		UpdatedAt:     time.Now(),
	}
	r.put(mapping)
	return mapping, nil
}

func (r *embyPlaybackRuntime) restartLocked(
	ctx context.Context,
	mapping *embyPlaybackMapping,
	positionMS int64,
	reason string,
) (*embyPlaybackMapping, error) {
	result, err := r.sessions.Restart(ctx, mapping.UserID, mapping.InternalID, service.PlaybackSessionRestartRequest{
		ProfileID:       mapping.ProfileID,
		StartPositionMS: positionMS,
		SubtitleTrack:   -1,
		MaxBitrate:      mapping.MaxBitrate,
		Reason:          reason,
	})
	if err != nil {
		return nil, err
	}
	ready, err := r.waitReady(ctx, mapping.UserID, result)
	if err != nil {
		return nil, err
	}
	mapping.GenerationID = ready.Session.CurrentGenerationID
	mapping.GenerationPos = positionMS
	mapping.LastPosition = positionMS
	mapping.ProfileID = generationProfile(ready, mapping.ProfileID)
	mapping.UpdatedAt = time.Now()
	r.put(mapping)
	return mapping, nil
}

func (r *embyPlaybackRuntime) waitReady(
	ctx context.Context,
	userID string,
	result service.PlaybackSessionResult,
) (service.PlaybackSessionResult, error) {
	if result.FirstSegmentReady && result.PlaylistURL != "" {
		return result, nil
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, embyPlaybackReadyTimeout)
	defer cancel()
	current := result
	for {
		if current.Session.State == playbacksession.SessionStateFailed ||
			current.Session.State == playbacksession.SessionStateClosed ||
			current.Session.State == playbacksession.SessionStateExpired {
			if current.Session.Generation != nil && current.Session.Generation.ErrorMessage != "" {
				return service.PlaybackSessionResult{}, errors.New(current.Session.Generation.ErrorMessage)
			}
			return service.PlaybackSessionResult{}, fmt.Errorf("playback session entered %s", current.Session.State)
		}
		select {
		case <-deadlineCtx.Done():
			return service.PlaybackSessionResult{}, deadlineCtx.Err()
		case <-time.After(embyPlaybackPollInterval):
		}
		status, err := r.sessions.Status(userID, current.Session.ID)
		if err != nil {
			return service.PlaybackSessionResult{}, err
		}
		current = status
		if current.FirstSegmentReady && current.PlaylistURL != "" {
			return current, nil
		}
	}
}

func generationProfile(result service.PlaybackSessionResult, fallback string) string {
	if result.Session.Generation != nil && result.Session.Generation.ProfileID != "" {
		return result.Session.Generation.ProfileID
	}
	return fallback
}

func (r *embyPlaybackRuntime) heartbeat(
	ctx context.Context,
	mapping *embyPlaybackMapping,
	positionMS int64,
	paused bool,
) error {
	if r == nil || mapping == nil {
		return nil
	}
	key := embyPlaybackKey(mapping.UserID, mapping.ExternalID)
	unlock := r.lockKey(key)
	defer unlock()
	current, ok := r.get(mapping.UserID, mapping.ExternalID)
	if !ok {
		return nil
	}

	if current.LastPosition > 0 {
		delta := time.Duration(positionMS-current.LastPosition) * time.Millisecond
		if delta < -embyPlaybackSeekBack || delta > embyPlaybackSeekForward {
			restarted, err := r.restartLocked(ctx, current, positionMS, "emby_progress_seek")
			if err != nil {
				return err
			}
			current = restarted
		}
	}

	_, err := r.sessions.Heartbeat(current.UserID, current.InternalID, service.PlaybackSessionHeartbeatRequest{
		GenerationID:  current.GenerationID,
		PositionMS:    positionMS,
		BufferedEndMS: positionMS + int64(embyPlaybackBufferedWindow/time.Millisecond),
		Paused:        paused,
	})
	if err != nil {
		return err
	}
	current.LastPosition = positionMS
	current.UpdatedAt = time.Now()
	r.put(current)
	return nil
}

func (r *embyPlaybackRuntime) close(ctx context.Context, mapping *embyPlaybackMapping, reason string) error {
	if r == nil || mapping == nil {
		return nil
	}
	key := embyPlaybackKey(mapping.UserID, mapping.ExternalID)
	unlock := r.lockKey(key)
	defer unlock()
	current, ok := r.get(mapping.UserID, mapping.ExternalID)
	if !ok {
		return nil
	}
	err := r.closeLocked(ctx, current, reason)
	r.delete(current.UserID, current.ExternalID)
	return err
}

func (r *embyPlaybackRuntime) closeLocked(ctx context.Context, mapping *embyPlaybackMapping, reason string) error {
	return r.sessions.Close(ctx, mapping.UserID, mapping.InternalID, reason)
}

func (r *embyPlaybackRuntime) openPlaylist(mapping *embyPlaybackMapping, generationID uint64) ([]byte, error) {
	file, err := r.sessions.OpenPlaylist(mapping.UserID, mapping.InternalID, generationID)
	if err != nil {
		return nil, err
	}
	defer file.Release()
	return os.ReadFile(file.Path)
}

func (r *embyPlaybackRuntime) openSegment(mapping *embyPlaybackMapping, generationID uint64, name string) (*service.PlaybackSessionFile, error) {
	return r.sessions.OpenSegment(mapping.UserID, mapping.InternalID, generationID, name)
}

func parseEmbyPlaybackRequest(c *gin.Context) (externalID string, startPositionMS int64, hasStart bool, maxBitrate int) {
	externalID = firstQuery(c, "PlaySessionId", "playSessionId", "playsessionid")
	if raw := firstQuery(c, "StartTimeTicks", "startTimeTicks", "starttimeticks"); raw != "" {
		ticks, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			startPositionMS = ticks / 10_000
			hasStart = true
		}
	}
	if raw := firstQuery(c, "maxBitrate", "MaxStreamingBitrate", "maxStreamingBitrate"); raw != "" {
		maxBitrate = atoiSafe(raw)
	}
	return
}

func firstQuery(c *gin.Context, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(c.Query(name)); value != "" {
			return value
		}
	}
	return ""
}

func embyRoutePrefix(c *gin.Context) string {
	if c != nil && c.Request != nil && strings.HasPrefix(strings.ToLower(c.Request.URL.Path), "/emby/") {
		return "/emby"
	}
	return ""
}

func embySessionPlaylistURL(c *gin.Context, embyID, externalID string, generationID uint64) string {
	result := fmt.Sprintf(
		"%s/Videos/%s/session/%s/%d/stream.m3u8",
		embyRoutePrefix(c),
		url.PathEscape(embyID),
		url.PathEscape(externalID),
		generationID,
	)
	if token, _ := extractToken(c); token != "" {
		result += "?api_key=" + url.QueryEscape(token)
	}
	return result
}

func embySessionSegmentURL(c *gin.Context, embyID, externalID string, generationID uint64, name string) string {
	result := fmt.Sprintf(
		"%s/Videos/%s/session/%s/%d/%s",
		embyRoutePrefix(c),
		url.PathEscape(embyID),
		url.PathEscape(externalID),
		generationID,
		url.PathEscape(name),
	)
	if token, _ := extractToken(c); token != "" {
		result += "?api_key=" + url.QueryEscape(token)
	}
	return result
}

func buildEmbySessionMaster(c *gin.Context, embyID string, mapping *embyPlaybackMapping) string {
	bandwidth := mapping.MaxBitrate
	if bandwidth <= 0 {
		bandwidth = 8_000_000
	}
	return strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-VERSION:3",
		fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d", bandwidth),
		embySessionPlaylistURL(c, embyID, mapping.ExternalID, mapping.GenerationID),
		"",
	}, "\n")
}

func rewriteEmbySessionPlaylist(
	c *gin.Context,
	embyID string,
	mapping *embyPlaybackMapping,
	generationID uint64,
	playlist []byte,
) string {
	lines := strings.Split(string(playlist), "\n")
	for index, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := url.Parse(line)
		if err != nil {
			continue
		}
		name := path.Base(parsed.Path)
		if name == "." || name == "/" || name == "" {
			continue
		}
		lines[index] = embySessionSegmentURL(c, embyID, mapping.ExternalID, generationID, name)
	}
	return strings.Join(lines, "\n")
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
