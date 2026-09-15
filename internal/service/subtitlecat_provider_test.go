package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func readSubtitleCatFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "subtitlecat", name))
	require.NoError(t, err)
	return data
}

func TestParseSubtitleCatSearchHTML(t *testing.T) {
	items, err := parseSubtitleCatSearchHTML(readSubtitleCatFixture(t, "search.html"))
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "Interstellar.2014.720p.BluRay.x264.YIFY", items[0].Title)
	assert.Equal(t, "/subs/18/Interstellar.2014.720p.BluRay.x264.YIFY.html", items[0].DetailPath)
	assert.Equal(t, "149 KB", items[0].FileSize)
	assert.Equal(t, 1878, items[0].DownloadCount)
	assert.Equal(t, 103, items[0].LanguageCount)
}

func TestParseSubtitleCatSearchHTMLRelativeDetailLinks(t *testing.T) {
	body := []byte(`<html><body><table><tr>
		<td><a href="subs/801/JUNY-146-SH-OCR.en-zh-CN.html">JUNY-146-SH-OCR.en-zh-CN</a></td>
		<td>SIZE 68 KB</td><td>14 downloads</td><td>14 languages</td>
	</tr><tr>
		<td><a href="./subs/802/JUNY-146-SH-OCR.en.html">JUNY-146-SH-OCR.en</a></td>
		<td>SIZE 69 KB</td><td>23 downloads</td><td>23 languages</td>
	</tr></table></body></html>`)

	items, err := parseSubtitleCatSearchHTML(body)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "/subs/801/JUNY-146-SH-OCR.en-zh-CN.html", items[0].DetailPath)
	assert.Equal(t, "/subs/802/JUNY-146-SH-OCR.en.html", items[1].DetailPath)
}

func TestParseSubtitleCatDetailHTMLLanguagePriority(t *testing.T) {
	provider := NewSubtitleCatProvider(zap.NewNop().Sugar())
	pageURL, err := url.Parse("https://www.subtitlecat.com/subs/18/Interstellar.2014.720p.BluRay.x264.YIFY.html")
	require.NoError(t, err)
	details, err := parseSubtitleCatDetailHTML(readSubtitleCatFixture(t, "detail.html"), pageURL, provider)
	require.NoError(t, err)
	require.Len(t, details.Languages, 4)
	assert.Equal(t, "zh-CN", details.Languages[0].Code)
	assert.Equal(t, "简体中文", details.Languages[0].Name)
	assert.Equal(t, "zh-TW", details.Languages[1].Code)
	assert.Equal(t, "en", details.Languages[2].Code)
	assert.Equal(t, "ja", details.Languages[3].Code)
	for _, lang := range details.Languages {
		assert.True(t, strings.HasPrefix(lang.DownloadID, "subtitlecat:download:"))
	}
}

func TestBuildSubtitleSearchQueriesMovie(t *testing.T) {
	queries := BuildSubtitleSearchQueries(
		"/media/Interstellar.2014.2160p.BluRay.x265.mkv",
		"",
		0,
		"movie",
	)
	require.NotEmpty(t, queries)
	assert.Contains(t, queries, "Interstellar 2014")
	assert.Contains(t, queries, "Interstellar.2014")
}

func TestBuildSubtitleSearchQueriesEpisodeKeepsSeasonEpisode(t *testing.T) {
	queries := BuildSubtitleSearchQueries(
		"/media/The.Last.of.Us.S02E03.2160p.WEB-DL.DDP5.1.H.265.mkv",
		"",
		0,
		"episode",
	)
	require.NotEmpty(t, queries)
	joined := strings.Join(queries, "\n")
	assert.Contains(t, strings.ToUpper(joined), "S02E03")
	assert.Contains(t, strings.ToLower(joined), "the last of us")
}

func TestSubtitleCatScoreRejectsWrongEpisode(t *testing.T) {
	req := SubtitleProviderSearchRequest{
		FileName:  "The.Last.of.Us.S01E02.1080p.WEB-DL.mkv",
		Title:     "The Last of Us",
		MediaType: "episode",
	}
	assert.Greater(t, scoreSubtitleCatCandidate("The.Last.of.Us.S01E02.1080p.WEB-DL", req), 50)
	assert.Equal(t, -1, scoreSubtitleCatCandidate("The.Last.of.Us.S01E03.1080p.WEB-DL", req))
}

func TestSubtitleCatSearchAndDownloadWithFixtureServer(t *testing.T) {
	searchFixture := readSubtitleCatFixture(t, "search.html")
	detailFixture := readSubtitleCatFixture(t, "detail.html")
	srtFixture := readSubtitleCatFixture(t, "sample.srt")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.php":
			assert.NotEmpty(t, r.URL.Query().Get("search"))
			assert.Contains(t, r.UserAgent(), "Mozilla/5.0")
			assert.Contains(t, r.UserAgent(), "Chrome/")
			assert.Contains(t, r.Header.Get("Accept-Language"), "en")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(searchFixture)
		case strings.HasSuffix(r.URL.Path, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(detailFixture)
		case strings.HasSuffix(r.URL.Path, ".srt"):
			w.Header().Set("Content-Type", "application/x-subrip")
			_, _ = w.Write(srtFixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := newSubtitleCatProviderForTest(server.URL, zap.NewNop().Sugar())
	require.NoError(t, err)
	results, err := provider.Search(context.Background(), SubtitleProviderSearchRequest{
		Queries:   []string{"Interstellar 2014"},
		FileName:  "Interstellar.2014.2160p.BluRay.x265.mkv",
		Title:     "Interstellar",
		Year:      2014,
		MediaType: "movie",
		Languages: []string{"zh-CN", "zh-TW", "en"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "subtitlecat", results[0].Source)
	assert.Equal(t, "zh-CN", results[0].Language)
	assert.GreaterOrEqual(t, results[0].MatchScore, results[len(results)-1].MatchScore)

	download, err := provider.Download(context.Background(), results[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "zh-CN", download.Language)
	assert.Contains(t, string(download.Content), "Hello from SubtitleCat")
}

func TestSubtitleCatProviderHTTPFailures(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()
			provider, err := newSubtitleCatProviderForTest(server.URL, zap.NewNop().Sugar())
			require.NoError(t, err)
			_, err = provider.Search(context.Background(), SubtitleProviderSearchRequest{Queries: []string{"Interstellar"}})
			require.Error(t, err)
			assert.True(t, IsSubtitleProviderUnavailable(err))
		})
	}
}

func TestSubtitleCatProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(120 * time.Millisecond)
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer server.Close()
	provider, err := newSubtitleCatProviderForTest(server.URL, zap.NewNop().Sugar())
	require.NoError(t, err)
	provider.client.Timeout = 20 * time.Millisecond
	_, err = provider.Search(context.Background(), SubtitleProviderSearchRequest{Queries: []string{"Interstellar"}})
	require.Error(t, err)
	assert.True(t, IsSubtitleProviderUnavailable(err))
}

func TestSubtitleCatParserStructureChange(t *testing.T) {
	_, err := parseSubtitleCatSearchHTML([]byte(`<html><body><div>layout changed</div></body></html>`))
	require.Error(t, err)
}

func TestSubtitleCatRejectsHTMLAsSRT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html><body>error</body></html>"))
	}))
	defer server.Close()
	provider, err := newSubtitleCatProviderForTest(server.URL, zap.NewNop().Sugar())
	require.NoError(t, err)
	id := encodeSubtitleCatRef("download", "/subs/801/test-zh-CN.srt")
	_, err = provider.Download(context.Background(), id)
	require.Error(t, err)
}

func TestNormalizeDownloadedSRTToUTF8(t *testing.T) {
	fixture := readSubtitleCatFixture(t, "sample.srt")
	normalized, err := normalizeDownloadedSRT(fixture)
	require.NoError(t, err)
	assert.NotContains(t, string(normalized), "\r")
	assert.True(t, strings.HasSuffix(string(normalized), "\n"))

	gbk, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), fixture)
	require.NoError(t, err)
	converted, err := normalizeDownloadedSRT(gbk)
	require.NoError(t, err)
	assert.Contains(t, string(converted), "字幕测试")
}

func TestSaveSubtitleCatSidecarPersistsNextToVideo(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "Interstellar.2014.mkv")
	require.NoError(t, os.WriteFile(videoPath, []byte("video"), 0644))
	service := NewSubtitleSearchService("", filepath.Join(dir, "cache"), zap.NewNop().Sugar())
	content, err := normalizeDownloadedSRT(readSubtitleCatFixture(t, "sample.srt"))
	require.NoError(t, err)
	result, err := service.saveSubtitleCatSidecar(videoPath, "remote-zh-CN.srt", "zh-CN", content)
	require.NoError(t, err)
	assert.Equal(t, "Interstellar.2014.chs.subtitlecat.srt", result.FileName)
	assert.Equal(t, "zh-CN", result.Language)
	stored, err := os.ReadFile(result.FilePath)
	require.NoError(t, err)
	assert.Equal(t, string(content), string(stored))
}
