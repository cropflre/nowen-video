package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// These read-only calls never invoke the overwrite scraper, download artwork,
// write NFO, or mutate the catalog. Approval uses the captured server proposal.
func (s *MetadataService) FetchAgentMovieMetadata(ctx context.Context, id int) (*TMDbMovieDetail, error) {
	if id <= 0 {
		return nil, ErrAgentMetadataSource
	}
	var detail TMDbMovieDetail
	if err := s.agentTMDbGet(ctx, fmt.Sprintf("/3/movie/%d", id), nil, &detail); err != nil || detail.ID != id {
		return nil, ErrAgentMetadataSource
	}
	return &detail, nil
}
func (s *MetadataService) SearchAgentMovieMatches(ctx context.Context, query string, year int) ([]TMDbMovie, error) {
	query = strings.TrimSpace(query)
	if query == "" || len([]rune(query)) > 120 || (year != 0 && (year < 1888 || year > 2100)) {
		return nil, ErrAgentMatchInput
	}
	params := url.Values{"query": {query}, "include_adult": {"false"}, "page": {"1"}}
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}
	var result TMDbSearchResult
	if err := s.agentTMDbGet(ctx, "/3/search/movie", params, &result); err != nil {
		return nil, err
	}
	if len(result.Results) > 20 {
		result.Results = result.Results[:20]
	}
	return result.Results, nil
}
func (s *MetadataService) agentTMDbGet(ctx context.Context, route string, params url.Values, out interface{}) error {
	if s.cfg == nil || strings.TrimSpace(s.cfg.Secrets.TMDbAPIKey) == "" || s.client == nil {
		return ErrAgentMetadataSource
	}
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", s.cfg.Secrets.TMDbAPIKey)
	params.Set("language", "zh-CN")
	endpoint := s.getTMDbAPIBase() + route + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ErrAgentMetadataSource
	}
	setAPIHeaders(req)
	client := *s.client
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 3 || next.URL.Host != req.URL.Host || next.URL.Scheme != req.URL.Scheme {
			return http.ErrUseLastResponse
		}
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return ErrAgentMetadataSource
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ErrAgentMetadataSource
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return ErrAgentMetadataSource
	}
	if json.Unmarshal(raw, out) != nil {
		return ErrAgentMetadataSource
	}
	return nil
}
