// Package service 番号刮削任务持久化存储
// 功能：
//   - 将批量刮削任务结果写入 JSON 文件持久化（内存+磁盘双存储）
//   - 失败记录可查询、导出
//   - 服务重启后能加载最近的任务历史
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	taskStoreFileName    = "adult_scrape_tasks.json"
	taskStoreMaxTasks    = 100
	taskStoreMaxItemsPer = 500 // 每任务保留的明细数
)

// AdultTaskStore 任务持久化存储
type AdultTaskStore struct {
	mu       sync.RWMutex
	filePath string
	tasks    []*AdultBatchTask
}

// NewAdultTaskStore 创建任务持久化存储
// dir：数据目录（通常为 config.DataDir）
func NewAdultTaskStore(dir string) *AdultTaskStore {
	if dir == "" {
		dir = "./data"
	}
	_ = os.MkdirAll(dir, 0o755)
	store := &AdultTaskStore{
		filePath: filepath.Join(dir, taskStoreFileName),
		tasks:    make([]*AdultBatchTask, 0, taskStoreMaxTasks),
	}
	_ = store.Load()
	return store
}

// Save 持久化到文件
func (s *AdultTaskStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0o644)
}

// Load 从文件加载
func (s *AdultTaskStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var tasks []*AdultBatchTask
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return err
	}
	s.tasks = tasks
	return nil
}

// Record 记录一个已完成的任务（会截断明细并自动保存）
func (s *AdultTaskStore) Record(task *AdultBatchTask) {
	if task == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 截断明细
	if len(task.Results) > taskStoreMaxItemsPer {
		task.Results = task.Results[len(task.Results)-taskStoreMaxItemsPer:]
	}
	// 保留最近 N 条
	if len(s.tasks) >= taskStoreMaxTasks {
		s.tasks = s.tasks[1:]
	}
	s.tasks = append(s.tasks, task)
	// 异步持久化
	go func() {
		_ = s.Save()
	}()
}

// List 列出所有任务（按时间倒序）
func (s *AdultTaskStore) List() []*AdultBatchTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*AdultBatchTask, len(s.tasks))
	copy(out, s.tasks)
	// 倒序
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}

// Get 按任务 ID 查询
func (s *AdultTaskStore) Get(id string) *AdultBatchTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// FailedItems 查询所有失败的条目（可用于一键重试）
func (s *AdultTaskStore) FailedItems(lastDays int) []AdultBatchItemResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cutoff := time.Now().Add(-time.Duration(lastDays) * 24 * time.Hour)
	out := []AdultBatchItemResult{}
	for _, t := range s.tasks {
		for _, r := range t.Results {
			if r.Status == "failed" && (lastDays <= 0 || r.FinishedAt.After(cutoff)) {
				out = append(out, r)
			}
		}
	}
	return out
}

// Stats 聚合统计信息
type AdultTaskStats struct {
	TotalTasks     int            `json:"total_tasks"`
	TotalProcessed int            `json:"total_processed"`
	TotalSuccess   int            `json:"total_success"`
	TotalFailed    int            `json:"total_failed"`
	TotalSkipped   int            `json:"total_skipped"`
	BySource       map[string]int `json:"by_source"`      // 按数据源统计成功数
	TopFailedCodes []string       `json:"top_failed_codes"`
}

// Stats 生成聚合统计
func (s *AdultTaskStore) Stats() AdultTaskStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := AdultTaskStats{
		BySource: map[string]int{},
	}
	failedCounter := map[string]int{}
	for _, t := range s.tasks {
		st.TotalTasks++
		for _, r := range t.Results {
			st.TotalProcessed++
			switch r.Status {
			case "success":
				st.TotalSuccess++
				if r.Source != "" {
					st.BySource[r.Source]++
				}
			case "failed":
				st.TotalFailed++
				if r.Code != "" {
					failedCounter[r.Code]++
				}
			case "skipped":
				st.TotalSkipped++
			}
		}
	}
	// 取失败次数最多的 10 个番号
	type pair struct {
		code  string
		count int
	}
	pairs := []pair{}
	for k, v := range failedCounter {
		pairs = append(pairs, pair{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	topN := 10
	if len(pairs) < topN {
		topN = len(pairs)
	}
	for i := 0; i < topN; i++ {
		st.TopFailedCodes = append(st.TopFailedCodes, pairs[i].code)
	}
	return st
}
