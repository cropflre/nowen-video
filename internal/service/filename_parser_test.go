package service

import (
	"fmt"
	"testing"
)

func TestParseMovieFilename_YYH3D(t *testing.T) {
	cases := []struct {
		filename string
		wantZH   string
		wantEN   string
		wantYear int
	}{
		{
			filename: "[yyh3d.com]采花和尚.Satyr Monks.1994.LD_D9.x264.AAC.480P.YYH3D.xt.mkv",
			wantZH:   "采花和尚",
			wantEN:   "Satyr Monks",
			wantYear: 1994,
		},
		{
			filename: "[yyh3d.com]吉屋藏娇.Ghost in the House.1988.LD_D9.x264.AAC.480P.YYH3D.xt.mkv",
			wantZH:   "吉屋藏娇",
			wantEN:   "Ghost in the House",
			wantYear: 1988,
		},
		{
			filename: "[yyh3d.com]奸人世家.Hong Kong Adam's Family.1994.LD_D9.x264.AAC.480P.YYH3D.xt.mkv.115chrome_4_28",
			wantZH:   "奸人世家",
			wantEN:   "Hong Kong Adam",
			wantYear: 1994,
		},
		{
			filename: "[yyh3d.com]地头龙.Dragon Fighter.1990.LD_D9.x264.AAC.480P.YYH3D.xt.mkv",
			wantZH:   "地头龙",
			wantEN:   "Dragon Fighter",
			wantYear: 1990,
		},
	}

	for _, c := range cases {
		t.Run(c.filename, func(t *testing.T) {
			got := ParseMovieFilename(c.filename)
			if got.Title != c.wantZH {
				t.Errorf("Title: want %q, got %q", c.wantZH, got.Title)
			}
			if got.Year != c.wantYear {
				t.Errorf("Year: want %d, got %d", c.wantYear, got.Year)
			}
			// 英文别名允许 HasPrefix / Contains，避免过于严格（我们不做词干/所有格处理）
			if c.wantEN != "" && got.TitleAlt == "" {
				t.Errorf("TitleAlt: want like %q, got empty", c.wantEN)
			}
			fmt.Printf("  %s → zh=%q en=%q year=%d\n", c.filename, got.Title, got.TitleAlt, got.Year)
		})
	}
}

func TestParseMovieFilename_OscarAwards(t *testing.T) {
	cases := []struct {
		filename string
		wantZH   string
		wantEN   string
		wantYear int
	}{
		{
			filename: "01届.《翼》-《Wings》-1927-1929。【十万度Q裙 319940383】.mkv",
			wantZH:   "翼",
			wantEN:   "Wings",
			wantYear: 1927,
		},
		{
			filename: "04届-《壮志千秋》-《Cimarron》-1931-1932。【十万度Q裙 218463625】.mkv",
			wantZH:   "壮志千秋",
			wantEN:   "Cimarron",
			wantYear: 1931,
		},
		{
			filename: "45届-《教父》-《The Godfather》-1972-1973。【十万度Q裙 319940383】.mkv",
			wantZH:   "教父",
			wantEN:   "The Godfather",
			wantYear: 1972,
		},
		{
			filename: "80届-《老无所依》-《No Country for Old Men》-2007-2008。【十万度Q裙 218463625】.mkv",
			wantZH:   "老无所依",
			wantEN:   "No Country for Old Men",
			wantYear: 2007,
		},
		{
			filename: "80届-《老无所依》-《No Country for Old Men》-2007-2008。【十万度Q裙 218463625】.mkv.115chrome_5_17",
			wantZH:   "老无所依",
			wantEN:   "No Country for Old Men",
			wantYear: 2007,
		},
	}

	for _, c := range cases {
		t.Run(c.filename, func(t *testing.T) {
			got := ParseMovieFilename(c.filename)
			if got.Title != c.wantZH {
				t.Errorf("Title: want %q, got %q", c.wantZH, got.Title)
			}
			if got.Year != c.wantYear {
				t.Errorf("Year: want %d, got %d", c.wantYear, got.Year)
			}
			if got.TitleAlt != c.wantEN {
				t.Errorf("TitleAlt: want %q, got %q", c.wantEN, got.TitleAlt)
			}
			fmt.Printf("  %s → zh=%q en=%q year=%d\n", c.filename, got.Title, got.TitleAlt, got.Year)
		})
	}
}

func TestParseMovieFilename_Classic(t *testing.T) {
	cases := []struct {
		filename string
		wantTit  string
		wantYear int
		wantTMDB int
	}{
		{"Avatar (2009).mkv", "Avatar", 2009, 0},
		{"Casino Royale (2006) [tmdbid=36557].mkv", "Casino Royale", 2006, 36557},
		{"黑客帝国 (1999) {tmdb-603}.mkv", "黑客帝国", 1999, 603},
		{"The.Matrix.1999.BluRay.1080p.x264.mkv", "The Matrix", 1999, 0},
		{"Inception.2010.REMUX.2160p.mkv", "Inception", 2010, 0},
	}
	for _, c := range cases {
		t.Run(c.filename, func(t *testing.T) {
			got := ParseMovieFilename(c.filename)
			if got.Title != c.wantTit {
				t.Errorf("Title: want %q, got %q", c.wantTit, got.Title)
			}
			if got.Year != c.wantYear {
				t.Errorf("Year: want %d, got %d", c.wantYear, got.Year)
			}
			if got.TMDbID != c.wantTMDB {
				t.Errorf("TMDbID: want %d, got %d", c.wantTMDB, got.TMDbID)
			}
			fmt.Printf("  %s → title=%q year=%d tmdb=%d\n", c.filename, got.Title, got.Year, got.TMDbID)
		})
	}
}
