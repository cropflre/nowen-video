package handler

import "testing"

func TestValidMediaAnalysisThumbnail(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xdb, 0x00}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	webp := []byte{'R', 'I', 'F', 'F', 0x04, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P'}

	cases := []struct {
		name string
		mime string
		data []byte
		want bool
	}{
		{name: "JPEG", mime: "image/jpeg", data: jpeg, want: true},
		{name: "JPG", mime: "image/jpg", data: jpeg, want: true},
		{name: "PNG", mime: "image/png", data: png, want: true},
		{name: "WebP", mime: "image/webp", data: webp, want: true},
		{name: "伪装 JPEG", mime: "image/jpeg", data: []byte("not-image"), want: false},
		{name: "MIME 与内容不一致", mime: "image/png", data: jpeg, want: false},
		{name: "未知格式", mime: "application/octet-stream", data: webp, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validMediaAnalysisThumbnail(tc.mime, tc.data); got != tc.want {
				t.Fatalf("validMediaAnalysisThumbnail(%q) = %v, want %v", tc.mime, got, tc.want)
			}
		})
	}
}
