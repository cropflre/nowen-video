package emby

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestEmbyVideoDirectStaticRangeServing(t *testing.T) {
	r, h, db, token := newCompatEndpointTestRouter(t)

	tmpDir := t.TempDir()
	mkvPath := filepath.Join(tmpDir, "sample.mkv")
	payload := []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if err := os.WriteFile(mkvPath, payload, 0o600); err != nil {
		t.Fatalf("write sample mkv: %v", err)
	}

	media := &model.Media{
		ID:         "direct-mkv-movie",
		LibraryID:  "compat-library",
		Title:      "Direct MKV Movie",
		FilePath:   mkvPath,
		FileSize:   int64(len(payload)),
		MediaType:  "movie",
		VideoCodec: "h264",
		AudioCodec: "aac",
	}
	if err := db.Create(media).Error; err != nil {
		t.Fatalf("create media: %v", err)
	}
	embyID := h.idMap.ToEmbyID(media.ID)

	cases := []struct {
		name      string
		method    string
		path      string
		rangeHdr  string
		wantCode  int
		wantBody  string
		wantRange string
	}{
		{
			name:      "static=true lowercase supports 206 Range",
			method:    http.MethodGet,
			path:      "/emby/Videos/" + embyID + "/stream?static=true",
			rangeHdr:  "bytes=0-15",
			wantCode:  http.StatusPartialContent,
			wantBody:  string(payload[0:16]),
			wantRange: "bytes 0-15/62",
		},
		{
			name:      "Infuse stream.mkv Static=true supports 206 Range",
			method:    http.MethodGet,
			path:      "/Videos/" + embyID + "/stream.mkv?Static=true",
			rangeHdr:  "bytes=10-19",
			wantCode:  http.StatusPartialContent,
			wantBody:  string(payload[10:20]),
			wantRange: "bytes 10-19/62",
		},
		{
			name:      "stream.mkv matching container serves raw file with 206 Range",
			method:    http.MethodGet,
			path:      "/Videos/" + embyID + "/stream.mkv",
			rangeHdr:  "bytes=4-11",
			wantCode:  http.StatusPartialContent,
			wantBody:  string(payload[4:12]),
			wantRange: "bytes 4-11/62",
		},
		{
			name:      "original endpoint always serves raw file with 206 Range",
			method:    http.MethodGet,
			path:      "/emby/Videos/" + embyID + "/original",
			rangeHdr:  "bytes=0-7",
			wantCode:  http.StatusPartialContent,
			wantBody:  string(payload[0:8]),
			wantRange: "bytes 0-7/62",
		},
		{
			name:     "HEAD original returns 200 with Accept-Ranges",
			method:   http.MethodHead,
			path:     "/emby/Videos/" + embyID + "/original",
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Emby-Token", token)
			if tc.rangeHdr != "" {
				req.Header.Set("Range", tc.rangeHdr)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status=%d, want %d; body=%q", rec.Code, tc.wantCode, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "video/x-matroska" {
				t.Fatalf("Content-Type=%q, want video/x-matroska", got)
			}
			if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
				t.Fatalf("Accept-Ranges=%q, want bytes", got)
			}
			if tc.wantRange != "" {
				if got := rec.Header().Get("Content-Range"); got != tc.wantRange {
					t.Fatalf("Content-Range=%q, want %q", got, tc.wantRange)
				}
			}
			if tc.method == http.MethodGet && rec.Body.String() != tc.wantBody {
				t.Fatalf("body=%q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
