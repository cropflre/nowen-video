package service

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestParseEpisodeInfo(t *testing.T) {
	s := &ScannerService{}

	tests := []struct {
		filename       string
		wantSeasonNum  int
		wantEpisodeNum int
		wantTitle      string // 期望的 episode_title，默认为空字符串
		desc           string
	}{
		// === 用户要求的格式 ===
		{
			filename:       "[异域字幕组][一拳超人][One-Punch Man][01][1280x720][简体].mkv",
			wantSeasonNum:  0, // 文件名中无季号，由 collectEpisodes 默认为1
			wantEpisodeNum: 1,
			desc:           "字幕组标准格式：方括号内纯数字",
		},
		{
			filename:       "[异域字幕组][一拳超人][One-Punch Man][12][1280x720][简体].mkv",
			wantSeasonNum:  0,
			wantEpisodeNum: 12,
			desc:           "字幕组标准格式：第12集",
		},
		{
			filename:       "[HYSUB][ONE PUNCH MAN S2][OVA01][GB_MP4][1280X720].mp4",
			wantSeasonNum:  2, // 从文件名中的 S2 提取
			wantEpisodeNum: 1,
			desc:           "变体格式：含 S2 季号 + OVA01",
		},
		{
			filename:       "[HYSUB][ONE PUNCH MAN S2][OVA03][GB_MP4][1280X720].mp4",
			wantSeasonNum:  2,
			wantEpisodeNum: 3,
			desc:           "变体格式：OVA03",
		},

		// === 标准 SxxExx 格式 ===
		{
			filename:       "One.Punch.Man.S01E01.720p.mkv",
			wantSeasonNum:  1,
			wantEpisodeNum: 1,
			desc:           "标准 S01E01 格式",
		},
		{
			filename:       "One.Punch.Man.S02E12.1080p.mkv",
			wantSeasonNum:  2,
			wantEpisodeNum: 12,
			desc:           "标准 S02E12 格式",
		},

		// === EP 格式 ===
		{
			filename:       "[字幕组] 一拳超人 EP05 [1080P].mkv",
			wantSeasonNum:  0,
			wantEpisodeNum: 5,
			desc:           "EP05 格式",
		},

		// === 第X集 格式 ===
		{
			filename:       "一拳超人 第3集.mkv",
			wantSeasonNum:  0,
			wantEpisodeNum: 3,
			desc:           "中文第X集格式",
		},

		// === 分辨率不应被误匹配 ===
		{
			filename:       "[字幕组][剧名][01][1920x1080][简体].mkv",
			wantSeasonNum:  0,
			wantEpisodeNum: 1,
			desc:           "1920x1080 不应被误匹配为集号",
		},
		{
			filename:       "[字幕组][剧名][05][720P].mkv",
			wantSeasonNum:  0,
			wantEpisodeNum: 5,
			desc:           "720P 不应影响[05]的正确匹配",
		},

		// === SP 特别篇 ===
		{
			filename:       "[字幕组][一拳超人][SP01][1080P].mkv",
			wantSeasonNum:  0,
			wantEpisodeNum: 1,
			desc:           "SP01 特别篇",
		},

		// === [数字END] 格式 ===
		{
			filename:       "[异域字幕组][一拳超人][One-Punch Man][12END][1280x720][简体].mp4",
			wantSeasonNum:  0,
			wantEpisodeNum: 12,
			desc:           "[12END] 格式：最后一集带END标记",
		},
		{
			filename:       "[HYSUB][ONE PUNCH MAN][24][GB_MP4][1280X720][END]-remux nvl.mp4",
			wantSeasonNum:  0,
			wantEpisodeNum: 24,
			wantTitle:      "", // -remux nvl 不应成为标题
			desc:           "[24][END] 格式：END在单独方括号中，技术标记不作为标题",
		},
		{
			filename:       "[HYSUB][ONE PUNCH MAN][13][GB_MP4][1280X720]-remux nvl.mp4",
			wantSeasonNum:  0,
			wantEpisodeNum: 13,
			wantTitle:      "", // -remux nvl 不应成为标题
			desc:           "技术标记 remux nvl 不应被识别为剧集标题",
		},
		{
			filename:       "[字幕组][动漫名][13FINAL][1080P].mkv",
			wantSeasonNum:  0,
			wantEpisodeNum: 13,
			desc:           "[13FINAL] 格式：FINAL后缀",
		},

		// === P1 新增：多集连播格式 ===
		{
			filename:       "Glee.S01E01-E02.720p.mkv",
			wantSeasonNum:  1,
			wantEpisodeNum: 1,
			desc:           "多集连播 S01E01-E02",
		},
		{
			filename:       "Show.S02E05-E08.1080p.mkv",
			wantSeasonNum:  2,
			wantEpisodeNum: 5,
			desc:           "多集连播 S02E05-E08",
		},
		{
			filename:       "Drama.S01E10-12.720p.mkv",
			wantSeasonNum:  1,
			wantEpisodeNum: 10,
			desc:           "多集连播 S01E10-12（无前缀E）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ep := s.parseEpisodeInfo(tt.filename)
			if ep.SeasonNum != tt.wantSeasonNum {
				t.Errorf("文件名: %s\n  季号: 期望 %d, 得到 %d", tt.filename, tt.wantSeasonNum, ep.SeasonNum)
			}
			if ep.EpisodeNum != tt.wantEpisodeNum {
				t.Errorf("文件名: %s\n  集号: 期望 %d, 得到 %d", tt.filename, tt.wantEpisodeNum, ep.EpisodeNum)
			}
			if ep.EpisodeTitle != tt.wantTitle {
				t.Errorf("文件名: %s\n  标题: 期望 %q, 得到 %q", tt.filename, tt.wantTitle, ep.EpisodeTitle)
			}
			fmt.Printf("  ✓ %s → S%02dE%02d (title=%q)\n", tt.filename, ep.SeasonNum, ep.EpisodeNum, ep.EpisodeTitle)
		})
	}
}

// TestNormalizeSeriesName 测试目录名标准化系列名提取
func TestNormalizeSeriesName(t *testing.T) {
	s := &ScannerService{}

	tests := []struct {
		dirName string
		want    string
		desc    string
	}{
		{"一拳超人 S1", "一拳超人", "中文名+S1"},
		{"一拳超人 S2", "一拳超人", "中文名+S2"},
		{"Breaking Bad Season 1", "Breaking Bad", "英文名+Season 1"},
		{"Breaking Bad Season 2", "Breaking Bad", "英文名+Season 2"},
		{"一拳超人 第一季", "一拳超人", "中文名+第一季"},
		{"一拳超人 第二季", "一拳超人", "中文名+第二季"},
		{"一拳超人 第2季", "一拳超人", "中文名+第2季（数字）"},
		{"一拳超人", "一拳超人", "无季号标识"},
		{"Attack on Titan S3", "Attack on Titan", "英文名+S3"},
		{"进击的巨人 第三季", "进击的巨人", "中文名+第三季"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := s.normalizeSeriesName(tt.dirName)
			if got != tt.want {
				t.Errorf("目录名: %q\n  期望: %q, 得到: %q", tt.dirName, tt.want, got)
			}
			fmt.Printf("  ✓ %q → %q\n", tt.dirName, got)
		})
	}
}

// TestExtractSeasonFromDirName 测试从目录名提取季号
func TestExtractSeasonFromDirName(t *testing.T) {
	s := &ScannerService{}

	tests := []struct {
		dirName string
		want    int
		desc    string
	}{
		{"一拳超人 S1", 1, "S1"},
		{"一拳超人 S02", 2, "S02"},
		{"Breaking Bad Season 3", 3, "Season 3"},
		{"一拳超人 第一季", 1, "第一季"},
		{"一拳超人 第二季", 2, "第二季"},
		{"一拳超人 第2季", 2, "第2季"},
		{"一拳超人", 0, "无季号"},
		{"Attack on Titan S10", 10, "S10"},
		{"进击的巨人 第三季", 3, "第三季"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := s.extractSeasonFromDirName(tt.dirName)
			if got != tt.want {
				t.Errorf("目录名: %q\n  季号: 期望 %d, 得到 %d", tt.dirName, tt.want, got)
			}
			fmt.Printf("  ✓ %q → 季号 %d\n", tt.dirName, got)
		})
	}
}

// TestExtractTitleEnhanced 测试增强的标题提取（P0: 年份+ID标签）
func TestExtractTitleEnhanced(t *testing.T) {
	s := &ScannerService{}

	tests := []struct {
		filename   string
		wantTitle  string
		wantYear   int
		wantTMDbID int
		desc       string
	}{
		{
			filename:  "Avatar (2009).mkv",
			wantTitle: "Avatar",
			wantYear:  2009,
			desc:      "标准 Emby 格式：片名 (年份)",
		},
		{
			filename:   "Casino Royale (2006) [tmdbid=36557].mkv",
			wantTitle:  "Casino Royale",
			wantYear:   2006,
			wantTMDbID: 36557,
			desc:       "Emby 格式带 TMDb ID 标签",
		},
		{
			filename:   "黑客帝国 (1999) {tmdb-603}.mkv",
			wantTitle:  "黑客帝国",
			wantYear:   1999,
			wantTMDbID: 603,
			desc:       "花括号 TMDb ID 格式",
		},
		{
			filename:  "The.Matrix.1999.BluRay.1080p.x264.mkv",
			wantTitle: "The Matrix",
			wantYear:  1999, // 增强后：支持从点分隔中提取年份
			desc:      "点分隔带编码标记的文件名（增强后能提取年份）",
		},
		{
			filename:  "黑客帝国 (1999).mkv",
			wantTitle: "黑客帝国",
			wantYear:  1999,
			desc:      "中文片名+年份",
		},
		{
			filename:  "Inception.2010.REMUX.2160p.mkv",
			wantTitle: "Inception",
			wantYear:  2010, // 增强后：支持从点分隔中提取年份
			desc:      "REMUX/2160p 标记应被清理",
		},
		{
			filename:  "简单的文件名.mkv",
			wantTitle: "简单的文件名",
			wantYear:  0,
			desc:      "无特殊标记的简单文件名",
		},
		{
			filename:  "Movie.Name.WEB-DL.720p.AAC.x265.mkv",
			wantTitle: "Movie Name",
			wantYear:  0,
			desc:      "多个编码标记应全部清理",
		},
		// ===== 子标题保留（防止"蜡笔小新：XXX" 被截成"蜡笔小新"，导致全部命中同一 TMDb 条目）=====
		{
			filename:  "蜡笔小新：灼热的春日部舞者.2025.mkv",
			wantTitle: "蜡笔小新: 灼热的春日部舞者",
			wantYear:  2025,
			desc:      "蜡笔小新+冒号子标题（必须保留子标题，否则会全部刮成同一部）",
		},
		{
			filename:  "蜡笔小新：云黑斋的野心 (1995).mkv",
			wantTitle: "蜡笔小新: 云黑斋的野心",
			wantYear:  1995,
			desc:      "蜡笔小新剧场版1995（早期作品，子标题必须保留）",
		},
		{
			filename:  "哈利·波特与魔法石 (2001).mkv",
			wantTitle: "哈利·波特与魔法石",
			wantYear:  2001,
			desc:      "中点·分隔的人名标题应整体保留",
		},
		{
			filename:  "你的名字 Your Name 2016.mkv",
			wantTitle: "你的名字",
			wantYear:  2016,
			desc:      "中英并列：取中文为主标题，英文进 TitleAlt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			title, year, tmdbID := s.extractTitleEnhanced(tt.filename)
			if title != tt.wantTitle {
				t.Errorf("文件名: %s\n  标题: 期望 %q, 得到 %q", tt.filename, tt.wantTitle, title)
			}
			if year != tt.wantYear {
				t.Errorf("文件名: %s\n  年份: 期望 %d, 得到 %d", tt.filename, tt.wantYear, year)
			}
			if tmdbID != tt.wantTMDbID {
				t.Errorf("文件名: %s\n  TMDbID: 期望 %d, 得到 %d", tt.filename, tt.wantTMDbID, tmdbID)
			}
			fmt.Printf("  ✓ %s → title=%q year=%d tmdbID=%d\n", tt.filename, title, year, tmdbID)
		})
	}
}

// TestMultiEpisodeDetection 测试多集连播 EpisodeNumEnd 字段
func TestMultiEpisodeDetection(t *testing.T) {
	s := &ScannerService{}

	tests := []struct {
		filename      string
		wantEpStart   int
		wantEpEnd     int
		wantSeasonNum int
		desc          string
	}{
		{
			filename:      "Glee.S01E01-E02.720p.mkv",
			wantEpStart:   1,
			wantEpEnd:     2,
			wantSeasonNum: 1,
			desc:          "S01E01-E02",
		},
		{
			filename:      "Show.S02E05-E08.1080p.mkv",
			wantEpStart:   5,
			wantEpEnd:     8,
			wantSeasonNum: 2,
			desc:          "S02E05-E08",
		},
		{
			filename:      "Drama.S01E10-12.720p.mkv",
			wantEpStart:   10,
			wantEpEnd:     12,
			wantSeasonNum: 1,
			desc:          "S01E10-12 无前缀E",
		},
		{
			filename:      "One.Punch.Man.S01E05.720p.mkv",
			wantEpStart:   5,
			wantEpEnd:     0, // 单集
			wantSeasonNum: 1,
			desc:          "S01E05 单集（EpisodeNumEnd 应为 0）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ep := s.parseEpisodeInfo(tt.filename)
			if ep.EpisodeNum != tt.wantEpStart {
				t.Errorf("文件名: %s\n  起始集号: 期望 %d, 得到 %d", tt.filename, tt.wantEpStart, ep.EpisodeNum)
			}
			if ep.EpisodeNumEnd != tt.wantEpEnd {
				t.Errorf("文件名: %s\n  结束集号: 期望 %d, 得到 %d", tt.filename, tt.wantEpEnd, ep.EpisodeNumEnd)
			}
			if ep.SeasonNum != tt.wantSeasonNum {
				t.Errorf("文件名: %s\n  季号: 期望 %d, 得到 %d", tt.filename, tt.wantSeasonNum, ep.SeasonNum)
			}
			fmt.Printf("  ✓ %s → S%02dE%02d-E%02d\n", tt.filename, ep.SeasonNum, ep.EpisodeNum, ep.EpisodeNumEnd)
		})
	}
}

// TestDateEpisodeDetection 测试日期格式集号（脱口秀/日播剧）
func TestDateEpisodeDetection(t *testing.T) {
	s := &ScannerService{}

	tests := []struct {
		filename    string
		wantAirDate string
		wantEpNum   int
		desc        string
	}{
		{
			filename:    "Last.Week.Tonight.2024.01.15.720p.mkv",
			wantAirDate: "2024-01-15",
			wantEpNum:   115, // month*100 + day
			desc:        "日播格式 2024.01.15",
		},
		{
			filename:    "脱口秀大会.2023-08-20.1080p.mkv",
			wantAirDate: "2023-08-20",
			wantEpNum:   820,
			desc:        "中文日播格式 2023-08-20",
		},
		{
			filename:    "Show.S01E05.720p.mkv",
			wantAirDate: "", // 应优先使用 SxxExx 模式
			wantEpNum:   5,
			desc:        "有 SxxExx 时不应使用日期格式",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ep := s.parseEpisodeInfo(tt.filename)
			if ep.AirDate != tt.wantAirDate {
				t.Errorf("文件名: %s\n  AirDate: 期望 %q, 得到 %q", tt.filename, tt.wantAirDate, ep.AirDate)
			}
			if ep.EpisodeNum != tt.wantEpNum {
				t.Errorf("文件名: %s\n  集号: 期望 %d, 得到 %d", tt.filename, tt.wantEpNum, ep.EpisodeNum)
			}
			fmt.Printf("  ✓ %s → AirDate=%q EpNum=%d\n", tt.filename, ep.AirDate, ep.EpisodeNum)
		})
	}
}

// TestIsExtrasPath 测试非正片目录过滤
func TestIsExtrasPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
		desc string
	}{
		{"/movies/Avatar/extras/deleted-scene.mkv", true, "extras 目录"},
		{"/movies/Avatar/trailers/trailer1.mkv", true, "trailers 目录"},
		{"/movies/Avatar/behind the scenes/bts.mkv", true, "behind the scenes 目录"},
		{"/movies/Avatar/featurettes/making-of.mkv", true, "featurettes 目录"},
		{"/movies/Avatar/Avatar.mkv", false, "正常电影文件"},
		{"/tv/Breaking Bad/Season 1/S01E01.mkv", false, "正常剧集文件"},
		{"/movies/Avatar/sample/sample.mkv", true, "sample 目录"},
		{"/movies/Avatar/bonus/extras.mkv", true, "bonus 目录"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := isExtrasPath(tt.path)
			if got != tt.want {
				t.Errorf("路径: %s\n  期望 %v, 得到 %v", tt.path, tt.want, got)
			}
			fmt.Printf("  ✓ %s → isExtras=%v\n", tt.path, got)
		})
	}
}

// TestIsExtrasFile 测试非正片文件后缀过滤
func TestIsExtrasFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
		desc     string
	}{
		{"Avatar-behindthescenes.mkv", true, "-behindthescenes 后缀"},
		{"Avatar-trailer.mkv", true, "-trailer 后缀"},
		{"Avatar-deleted.mkv", true, "-deleted 后缀"},
		{"Avatar-featurette.mkv", true, "-featurette 后缀"},
		{"Avatar-sample.mkv", true, "-sample 后缀"},
		{"Avatar.mkv", false, "正常文件"},
		{"Avatar-part1.mkv", false, "正常多部分文件"},
		{"Interview with Director.mkv", false, "标题含 interview 但无后缀"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := isExtrasFile(tt.filename)
			if got != tt.want {
				t.Errorf("文件名: %s\n  期望 %v, 得到 %v", tt.filename, tt.want, got)
			}
			fmt.Printf("  ✓ %s → isExtrasFile=%v\n", tt.filename, got)
		})
	}
}

// TestSeasonZeroSupport 测试 Season 0 / Specials 目录支持
func TestSeasonZeroSupport(t *testing.T) {
	s := &ScannerService{}

	tests := []struct {
		dirName string
		want    int
		desc    string
	}{
		{"Season 0", 0, "Season 0"},
		{"Season 00", 0, "Season 00"},
		{"Specials", 0, "Specials"},
		{"Special", 0, "Special"},
		{"Season 1", 1, "Season 1 正常季"},
		{"Season 10", 10, "Season 10"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := s.extractSeasonFromDirName(tt.dirName)
			if got != tt.want {
				t.Errorf("目录名: %q\n  季号: 期望 %d, 得到 %d", tt.dirName, tt.want, got)
			}
			fmt.Printf("  ✓ %q → 季号 %d\n", tt.dirName, got)
		})
	}
}

// TestIsHiddenOrCacheDirName 隐藏目录与 NAS 缓存目录名判断
func TestIsHiddenOrCacheDirName(t *testing.T) {
	cases := map[string]bool{
		".@__thumb":         true, // 飞牛 fnOS 缩略图缓存
		"@eaDir":            true, // 群晖缩略图/元数据
		"#recycle":          true, // 群晖回收站
		".Trashes":          true, // macOS
		".Spotlight-V100":   true, // macOS 索引
		"$RECYCLE.BIN":      true, // Windows 回收站
		"System Volume Information": true,
		"Season 01":         false,
		"布鲁伊第一季":            false,
		"extras":            false, // 由 extrasExcludeDirs 单独处理
		"":                  false,
	}
	for name, want := range cases {
		if got := isHiddenOrCacheDirName(name); got != want {
			t.Errorf("isHiddenOrCacheDirName(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestPathHasHiddenOrCacheSegment 路径中任一段命中隐藏/缓存目录
func TestPathHasHiddenOrCacheSegment(t *testing.T) {
	if !pathHasHiddenOrCacheSegment(`/media/剧/.@__thumb/s800《剧》 第1集.mp4`) {
		t.Error("包含 .@__thumb 段的路径应命中")
	}
	if !pathHasHiddenOrCacheSegment(`/media/@eaDir/剧.mp4`) {
		t.Error("包含 @eaDir 段的路径应命中")
	}
	if pathHasHiddenOrCacheSegment(`/media/布鲁伊第一季/《剧》 第1集.mp4`) {
		t.Error("正常媒体路径不应命中")
	}
}

// TestCollectEpisodesSkipsHiddenCacheDirs 剧集收集必须跳过 NAS 缓存目录
// （回归：fnOS .@__thumb 内 JPEG 缩略图伪 .mp4 被当作剧集入库，产生 mjpeg 幽灵剧集）
func TestCollectEpisodesSkipsHiddenCacheDirs(t *testing.T) {
	s := &ScannerService{}
	root := t.TempDir()

	write := func(rel string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("S01E01.mp4")
	write("S01E02.mp4")
	write("子目录/S01E03.mp4")
	// fnOS 缩略图缓存：隐藏目录 + 保留原视频 .mp4 扩展名的 JPEG
	write(".@__thumb/s800《剧》 第1集.mp4")
	write(".@__thumb/s100《剧》 第2集.mp4")
	// 群晖 @eaDir
	write("@eaDir/S01E01.mp4")
	// 深层嵌套的隐藏目录
	write("子目录/.Spotlight-V100/S01E04.mp4")

	eps := s.collectEpisodes(root)
	if len(eps) != 3 {
		t.Fatalf("应只收集 3 个正片, 实际 %d 个", len(eps))
	}
	for _, ep := range eps {
		if pathHasHiddenOrCacheSegment(ep.FilePath) {
			t.Errorf("不应收集缓存目录内文件: %s", ep.FilePath)
		}
	}
}

// TestIsStillImageMedia 静态图片防御过滤（不得误杀正常时长的 Motion-JPEG 视频）
func TestIsStillImageMedia(t *testing.T) {
	cases := []struct {
		desc     string
		codec    string
		duration float64
		want     bool
	}{
		{"fnOS 缩略图(JPEG 内容, 单帧时长)", "mjpeg", 0.04, true},
		{"无时长的 JPEG", "mjpeg", 0, true},
		{"PNG 图片", "png", 0, true},
		{"正常时长的 Motion-JPEG 视频", "mjpeg", 1420.5, false},
		{"h264 正片", "h264", 1420.5, false},
		{"探测失败(codec 为空, 如 ffprobe 不可用)", "", 0, false},
	}
	for _, c := range cases {
		media := &model.Media{VideoCodec: c.codec, Duration: c.duration}
		if got := isStillImageMedia(media); got != c.want {
			t.Errorf("%s: isStillImageMedia(codec=%q, dur=%v) = %v, want %v", c.desc, c.codec, c.duration, got, c.want)
		}
	}
}
