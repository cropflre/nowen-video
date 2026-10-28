package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// A read-only alternative to ScrapeMedia/MatchMediaWithTMDb. The response is
// captured before approval; applying an approved patch requires no network.
func (s *MetadataService) FetchAgentMovieMetadata(ctx context.Context, id int) (*TMDbMovieDetail, error) {
	if id <= 0 || s.cfg == nil || strings.TrimSpace(s.cfg.Secrets.TMDbAPIKey) == "" || s.client == nil {
		return nil, ErrAgentMetadataSource
	}
	endpoint := fmt.Sprintf("%s/3/movie/%d?api_key=%s&language=zh-CN", s.getTMDbAPIBase(), id, url.QueryEscape(s.cfg.Secrets.TMDbAPIKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrAgentMetadataSource
	}
	setAPIHeaders(req)
	client := *s.client
	// Do not forward provider credentials to a different redirect destination.
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 3 || next.URL.Host != req.URL.Host || next.URL.Scheme != req.URL.Scheme {
			return http.ErrUseLastResponse
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrAgentMetadataSource
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrAgentMetadataSource
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return nil, ErrAgentMetadataSource
	}
	var detail TMDbMovieDetail
	if json.Unmarshal(raw, &detail) != nil || detail.ID != id {
		return nil, ErrAgentMetadataSource
	}
	return &detail, nil
}
