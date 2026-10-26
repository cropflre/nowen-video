package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
)

type techProbeCacheEntry struct {
	specs     *TechSpecs
	expiresAt time.Time
}

func (s *MediaService) probeTechSpecs(media *model.Media) *TechSpecs {
	if media == nil {
		return nil
	}
	if media.StreamURL == "" && !isSTRMFile(media.FilePath) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return s.probeTechSpecsSource(ctx, media.FilePath, nil)
	}
	// Respect the administrator's remote-probe opt-out, also on detail requests.
	if s.cfg == nil || !s.cfg.STRM.RemoteProbe {
		return nil
	}
	// Work on a copy: resolving an old, unpopulated STRM record must not change
	// persisted metadata or overwrite per-media header overrides.
	resolved := *media
	if strings.TrimSpace(resolved.StreamURL) == "" {
		meta, err := ParseSTRMFileEnhanced(resolved.FilePath)
		if err != nil {
			return nil
		}
		resolved.StreamURL = meta.URL
		if resolved.StreamUA == "" {
			resolved.StreamUA = meta.UserAgent
		}
		if resolved.StreamReferer == "" {
			resolved.StreamReferer = meta.Referer
		}
		if resolved.StreamCookie == "" {
			resolved.StreamCookie = meta.Cookie
		}
		headers := meta.Headers
		if headers == nil {
			headers = map[string]string{}
		}
		for k, v := range DecodeSTRMHeaders(resolved.StreamHeaders) {
			headers[k] = v
		}
		encoded, _ := json.Marshal(headers)
		resolved.StreamHeaders = string(encoded)
	}
	// Unlike scan-time heuristics, an explicit detail request can probe HLS or
	// extensionless HTTP endpoints, including redirecting cloud-drive links.
	u, err := url.Parse(strings.TrimSpace(resolved.StreamURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	resolved.StreamURL = u.String()
	headers := strmProbeHeaders(&s.cfg.STRM, &resolved)
	args := strmProbeHeaderArgs(headers)
	timeout := s.cfg.STRM.RemoteProbeTimeout
	if timeout <= 0 {
		timeout = 8
	}
	if timeout > 120 {
		timeout = 120
	}
	// Hash the resolved identity so signed URLs and cookies are not retained as
	// map keys. Header/config changes invalidate both success and failure entries.
	identity, _ := json.Marshal([]any{resolved.StreamURL, headers, s.cfg.App.FFprobePath, timeout})
	key := fmt.Sprintf("%x", sha256.Sum256(identity))
	value, _, _ := s.techFlights.Do(key, func() (any, error) {
		s.techProbeMu.Lock()
		if entry, ok := s.techProbes[key]; ok && time.Now().Before(entry.expiresAt) {
			s.techProbeMu.Unlock()
			return entry.specs, nil
		}
		s.techProbeMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()
		specs := s.probeTechSpecsSource(ctx, resolved.StreamURL, args)
		ttl := 10 * time.Minute
		if specs == nil {
			ttl = 30 * time.Second
		}
		s.techProbeMu.Lock()
		defer s.techProbeMu.Unlock()
		if s.techProbes == nil {
			s.techProbes = make(map[string]techProbeCacheEntry)
		}
		// Bound memory even when many distinct signed URLs are visited.
		if len(s.techProbes) >= 128 {
			oldestKey := ""
			var oldest time.Time
			for k, entry := range s.techProbes {
				if oldestKey == "" || entry.expiresAt.Before(oldest) {
					oldestKey, oldest = k, entry.expiresAt
				}
			}
			delete(s.techProbes, oldestKey)
		}
		s.techProbes[key] = techProbeCacheEntry{specs: specs, expiresAt: time.Now().Add(ttl)}
		return specs, nil
	})
	if value == nil {
		return nil
	}
	return value.(*TechSpecs)
}

// Keep scan-time and detail-time authentication identical.
func strmProbeHeaders(cfg *config.STRMConfig, media *model.Media) http.Header {
	ua, referer, cookie, extra := ResolveSTRMHeaders(cfg, media)
	headers := http.Header{}
	for k, v := range map[string]string{"User-Agent": ua, "Referer": referer, "Cookie": cookie} {
		if v != "" {
			headers.Set(k, v)
		}
	}
	for k, v := range extra {
		headers.Set(k, v)
	}
	return headers
}

func strmProbeHeaderArgs(headers http.Header) []string {
	var args []string
	if ua := headers.Get("User-Agent"); ua != "" {
		args = append(args, "-user_agent", ua)
	}
	var block strings.Builder
	for k, vs := range headers {
		if strings.EqualFold(k, "User-Agent") {
			continue
		}
		for _, v := range vs {
			block.WriteString(k + ": " + v + "\r\n")
		}
	}
	if block.Len() > 0 {
		args = append(args, "-headers", block.String())
	}
	return args
}
