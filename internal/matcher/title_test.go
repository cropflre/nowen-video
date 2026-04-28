package matcher

import "testing"

// TestExtractBaseNameDeep_ChineseMixedSuffix 覆盖"基础名+数字+连接词+副标题"场景。
func TestExtractBaseNameDeep_ChineseMixedSuffix(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"逃学威龙3之龙过鸡年", "逃学威龙"}, // 连接词 "之" + 尾部数字 3
		{"哈利波特2之消失的密室", "哈利波特"}, // 经典场景
		{"Iron Man 3: Rise of Ultron", "Iron Man"},
	}
	for _, c := range cases {
		got := ExtractBaseNameDeep(c.title)
		if got != c.want {
			t.Errorf("ExtractBaseNameDeep(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

// TestExtractSeriesBaseName_SimpleSequel 保留 L1 基本用例。
func TestExtractSeriesBaseName_SimpleSequel(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"逃学威龙2", "逃学威龙"},
		{"速度与激情7", "速度与激情"},
		{"Toy Story 2", "Toy Story"},
	}
	for _, c := range cases {
		got := ExtractSeriesBaseName(c.title)
		if got != c.want {
			t.Errorf("ExtractSeriesBaseName(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

// TestNormalizeForCompare 验证归一化忽略空白、标点、全半角差异。
func TestNormalizeForCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"逃学威龙", "逃学威龙 ", true},
		{"逃学威龙", "逃学 威龙", true},
		{"Toy Story", "toy story", true},
		{"Toy Story", "toy-story", true},
		{"逃学威龙", "逃学威", false},
	}
	for _, c := range cases {
		got := NormalizeForCompare(c.a) == NormalizeForCompare(c.b)
		if got != c.want {
			t.Errorf("NormalizeForCompare(%q)==(%q) got=%v want=%v", c.a, c.b, got, c.want)
		}
	}
}

// TestTaoxueweilongTrio 关键场景：三部"逃学威龙"应产生同一个 baseName。
func TestTaoxueweilongTrio(t *testing.T) {
	titles := []string{"逃学威龙", "逃学威龙2", "逃学威龙3之龙过鸡年"}
	keys := make(map[string]int)
	for _, t := range titles {
		// 模拟 AutoMatchCollections 的前置提取逻辑
		k := ExtractBaseNameDeep(t)
		if k == "" {
			k = ExtractSeriesBaseName(t)
		}
		// 首部电影三层全落空，此时用 NormalizeForCompare 与其它 key 比对
		if k == "" {
			k = t // 作为候选，由服务层的"裸标题吸附"阶段再归一化比对
		}
		keys[NormalizeForCompare(k)]++
	}
	// 第二、第三部应归到同一 key "逃学威龙"；首部独立
	normBase := NormalizeForCompare("逃学威龙")
	if keys[normBase] < 2 {
		t.Fatalf("期望至少 2 部电影归到 key %q，实际 keys=%v", normBase, keys)
	}
}
