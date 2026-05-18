package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestIngestJournalRoundtrip 验证 journal 写入 → 读取 → 回滚 完整链路。
func TestIngestJournalRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	dstDir := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 准备 3 个源文件
	files := []string{"a.mkv", "b.mkv", "c.mkv"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(srcDir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	jp := filepath.Join(tmp, "journal.log")
	jw, err := newIngestJournalWriter(jp)
	if err != nil {
		t.Fatalf("create journal: %v", err)
	}

	// 模拟 move：mkdir + rename × 3
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := jw.AppendMkdir(dstDir); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		src := filepath.Join(srcDir, f)
		dst := filepath.Join(dstDir, f)
		if err := jw.AppendRename(src, dst); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(src, dst); err != nil {
			t.Fatal(err)
		}
	}
	if err := jw.Close(); err != nil {
		t.Fatal(err)
	}

	// 读取 journal
	entries, corrupted, err := ReadIngestJournal(jp)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if corrupted != 0 {
		t.Errorf("expected 0 corrupted, got %d", corrupted)
	}
	if len(entries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(entries))
	}

	// 回滚
	res, err := RollbackIngestJournal(jp)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if res.RestoredMv != 3 {
		t.Errorf("expected 3 restored, got %d", res.RestoredMv)
	}

	// 校验文件已回到 src
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(srcDir, f)); err != nil {
			t.Errorf("file %s should be restored to src: %v", f, err)
		}
		if _, err := os.Stat(filepath.Join(dstDir, f)); err == nil {
			t.Errorf("file %s should NOT exist in dst after rollback", f)
		}
	}
}

// TestIngestJournalCorruptionTolerant 验证损坏行不会中断回滚。
func TestIngestJournalCorruptionTolerant(t *testing.T) {
	tmp := t.TempDir()
	jp := filepath.Join(tmp, "j.log")

	// 手工写入 1 行合法 + 1 行损坏 + 1 行合法
	srcA := filepath.Join(tmp, "a.txt")
	dstA := filepath.Join(tmp, "a.moved")
	srcB := filepath.Join(tmp, "b.txt")
	dstB := filepath.Join(tmp, "b.moved")
	_ = os.WriteFile(srcA, []byte("x"), 0o644)
	_ = os.WriteFile(srcB, []byte("y"), 0o644)
	_ = os.Rename(srcA, dstA)
	_ = os.Rename(srcB, dstB)

	// JSON 字符串中反斜杠须转义；统一转换为斜杠避免跨平台问题
	srcAj, _ := json.Marshal(srcA)
	dstAj, _ := json.Marshal(dstA)
	srcBj, _ := json.Marshal(srcB)
	dstBj, _ := json.Marshal(dstB)

	content := `{"op":"rename","src":` + string(srcAj) + `,"dst":` + string(dstAj) + `,"ts":"2026-01-01T00:00:00Z"}` + "\n" +
		`THIS_IS_GARBAGE` + "\n" +
		`{"op":"rename","src":` + string(srcBj) + `,"dst":` + string(dstBj) + `,"ts":"2026-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(jp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := RollbackIngestJournal(jp)
	if err != nil {
		t.Fatal(err)
	}
	if res.Corrupted != 1 {
		t.Errorf("expected 1 corrupted, got %d", res.Corrupted)
	}
	if res.RestoredMv != 2 {
		t.Errorf("expected 2 restored, got %d", res.RestoredMv)
	}
}

// TestIngestJournalSkipWhenDstMissing 验证 dst 不存在时跳过而不报错。
func TestIngestJournalSkipWhenDstMissing(t *testing.T) {
	tmp := t.TempDir()
	jp := filepath.Join(tmp, "j.log")
	jw, err := newIngestJournalWriter(jp)
	if err != nil {
		t.Fatal(err)
	}
	// 故意写一条 dst 不存在的记录
	if err := jw.AppendRename(filepath.Join(tmp, "nope_src"), filepath.Join(tmp, "nope_dst")); err != nil {
		t.Fatal(err)
	}
	_ = jw.Close()

	res, err := RollbackIngestJournal(jp)
	if err != nil {
		t.Fatal(err)
	}
	if res.SkippedMv != 1 {
		t.Errorf("expected 1 skipped, got %d", res.SkippedMv)
	}
	if res.RestoredMv != 0 {
		t.Errorf("expected 0 restored, got %d", res.RestoredMv)
	}
}
