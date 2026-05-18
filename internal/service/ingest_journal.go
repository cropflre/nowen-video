package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ==================== 一键入库 · Move 模式 Journal ====================
//
// 设计目标：在专家模式（Mode=move）下，每一次 os.Rename 之前先把"src→dst"操作
// 写入 journal（单文件 append-only，每行一条 JSON）。这样即使进程崩溃、机器掉电，
// 也能根据 journal 倒序还原文件位置。
//
// 文件格式（NDJSON，每行一条）：
//   {"op":"rename","src":"<old>","dst":"<new>","ts":"2026-..."}
//   {"op":"mkdir","path":"<dir>","ts":"..."}        // 仅记录 MkdirAll 创建过的非空新目录
//
// 回滚规则：
//   - rename: os.Rename(dst, src)；若 dst 不存在跳过；若 src 已存在跳过并记入 errors
//   - mkdir : 仅当目录为空时 os.Remove，否则保留
//   - 倒序逐条执行（确保父目录在子文件之后处理）
//
// 安全：
//   - 写 journal 用文件锁（独占 + sync）保证 append 原子性
//   - 回滚过程中再次写入 ".rollback" 后缀的反向 journal，便于审计

// IngestJournalEntry 一条 journal 记录
type IngestJournalEntry struct {
	Op   string    `json:"op"`             // rename | mkdir
	Src  string    `json:"src,omitempty"`  // rename 时：原路径（src）
	Dst  string    `json:"dst,omitempty"`  // rename 时：新路径（dst）
	Path string    `json:"path,omitempty"` // mkdir 时：目录路径
	TS   time.Time `json:"ts"`
}

// ingestJournalWriter append-only 写入器（按 jobID 单实例，串行写入）
type ingestJournalWriter struct {
	path string
	f    *os.File
	mu   sync.Mutex
}

// newIngestJournalWriter 创建/追加 journal 文件
func newIngestJournalWriter(path string) (*ingestJournalWriter, error) {
	if path == "" {
		return nil, errors.New("journal path 不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建 journal 目录失败: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开 journal 失败: %w", err)
	}
	return &ingestJournalWriter{path: path, f: f}, nil
}

// AppendRename 写一条 rename 记录（保证 fsync）
func (w *ingestJournalWriter) AppendRename(src, dst string) error {
	return w.append(IngestJournalEntry{Op: "rename", Src: src, Dst: dst, TS: time.Now()})
}

// AppendMkdir 写一条 mkdir 记录
func (w *ingestJournalWriter) AppendMkdir(path string) error {
	return w.append(IngestJournalEntry{Op: "mkdir", Path: path, TS: time.Now()})
}

func (w *ingestJournalWriter) append(e IngestJournalEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return errors.New("journal 已关闭")
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := w.f.Write(b); err != nil {
		return err
	}
	// 关键：fsync，确保崩溃后 journal 不丢
	return w.f.Sync()
}

// Close 关闭 journal
func (w *ingestJournalWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// ReadIngestJournal 读取并解析一个 journal 文件
//
// 返回原始顺序的所有条目。损坏的行会被跳过并记录到 corrupted 数量。
func ReadIngestJournal(path string) (entries []IngestJournalEntry, corrupted int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// 单行最大 1MB（路径再长也够用）
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e IngestJournalEntry
		if err := json.Unmarshal(line, &e); err != nil {
			corrupted++
			continue
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return entries, corrupted, err
	}
	return entries, corrupted, nil
}

// IngestRollbackResult 回滚结果统计
type IngestRollbackResult struct {
	Total      int      `json:"total"`       // journal 中的条目总数
	RestoredMv int      `json:"restored_mv"` // 成功还原的 rename 数
	SkippedMv  int      `json:"skipped_mv"`  // 因 dst 不存在或 src 已存在而跳过的 rename 数
	RemovedDir int      `json:"removed_dir"` // 成功移除的空目录数
	KeptDir    int      `json:"kept_dir"`    // 因非空保留的目录数
	Errors     []string `json:"errors"`      // 错误明细（不阻断流程）
	Corrupted  int      `json:"corrupted"`   // 损坏行数
}

// RollbackIngestJournal 倒序还原一个 journal
//
// 行为：
//   - 倒序遍历条目
//   - rename: 若 dst 存在且 src 不存在 -> os.Rename(dst, src) 还原
//   - mkdir : 若目录为空 -> os.Remove
//   - 完成后写出 ".rollback" 后缀的反向 journal（仅审计用，不影响功能）
//
// 注意：本函数不会修改 IngestJob 状态，调用方负责事后更新数据库字段。
func RollbackIngestJournal(path string) (*IngestRollbackResult, error) {
	entries, corrupted, err := ReadIngestJournal(path)
	if err != nil {
		return nil, err
	}
	res := &IngestRollbackResult{
		Total:     len(entries),
		Corrupted: corrupted,
	}

	// 倒序处理
	rev := make([]IngestJournalEntry, len(entries))
	copy(rev, entries)
	sort.SliceStable(rev, func(i, j int) bool { return i > j }) // 等价于反转

	// 反向 journal（审计）
	rbPath := path + ".rollback"
	rbW, _ := newIngestJournalWriter(rbPath)
	defer func() {
		if rbW != nil {
			_ = rbW.Close()
		}
	}()

	for _, e := range rev {
		switch e.Op {
		case "rename":
			// 把 dst 移回 src
			if e.Dst == "" || e.Src == "" {
				res.SkippedMv++
				continue
			}
			if _, err := os.Stat(e.Dst); err != nil {
				// dst 不存在 -> 大概率已经被人手动还原过
				res.SkippedMv++
				continue
			}
			if _, err := os.Stat(e.Src); err == nil {
				// src 已存在 -> 拒绝覆盖，记录错误
				res.Errors = append(res.Errors, fmt.Sprintf("源位置已被占用，跳过: %s", e.Src))
				res.SkippedMv++
				continue
			}
			// 确保 src 父目录存在
			if err := os.MkdirAll(filepath.Dir(e.Src), 0o755); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("无法重建父目录 %s: %v", filepath.Dir(e.Src), err))
				continue
			}
			if err := os.Rename(e.Dst, e.Src); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("还原失败 %s -> %s: %v", e.Dst, e.Src, err))
				continue
			}
			res.RestoredMv++
			if rbW != nil {
				_ = rbW.AppendRename(e.Dst, e.Src)
			}
		case "mkdir":
			if e.Path == "" {
				continue
			}
			if isEmptyDir(e.Path) {
				if err := os.Remove(e.Path); err != nil {
					res.KeptDir++
					continue
				}
				res.RemovedDir++
			} else {
				res.KeptDir++
			}
		}
	}
	return res, nil
}

// isEmptyDir 目录是否为空（不存在 / 非目录 / 读取失败 都返回 false）
func isEmptyDir(p string) bool {
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() {
		return false
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	names, err := f.Readdirnames(1)
	if err != nil {
		// io.EOF 等价于"空"
		return len(names) == 0
	}
	return len(names) == 0
}
