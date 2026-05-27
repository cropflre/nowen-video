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
	folderTaskStoreFileName    = "adult_folder_scrape_tasks.json"
	folderTaskStoreMaxTasks    = 100
	folderTaskStoreMaxItemsPer = 500
)

// AdultFolderTaskStore 文件夹懒人刮削任务持久化存储。
type AdultFolderTaskStore struct {
	mu       sync.RWMutex
	filePath string
	tasks    []*FolderBatchTask
}

func NewAdultFolderTaskStore(dir string) *AdultFolderTaskStore {
	if dir == "" {
		dir = "./data"
	}
	_ = os.MkdirAll(dir, 0o755)
	store := &AdultFolderTaskStore{
		filePath: filepath.Join(dir, folderTaskStoreFileName),
		tasks:    make([]*FolderBatchTask, 0, folderTaskStoreMaxTasks),
	}
	_ = store.Load()
	return store
}

func (s *AdultFolderTaskStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0o644)
}

func (s *AdultFolderTaskStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var tasks []*FolderBatchTask
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return err
	}
	s.tasks = tasks
	return nil
}

func (s *AdultFolderTaskStore) Record(task *FolderBatchTask) {
	if task == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(task.Results) > folderTaskStoreMaxItemsPer {
		task.Results = task.Results[len(task.Results)-folderTaskStoreMaxItemsPer:]
	}
	if len(s.tasks) >= folderTaskStoreMaxTasks {
		s.tasks = s.tasks[1:]
	}
	s.tasks = append(s.tasks, task)
	go func() { _ = s.Save() }()
}

func (s *AdultFolderTaskStore) List() []*FolderBatchTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*FolderBatchTask, len(s.tasks))
	copy(out, s.tasks)
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}

func (s *AdultFolderTaskStore) Get(id string) *FolderBatchTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, task := range s.tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

func (s *AdultFolderTaskStore) FailedItems(lastDays int) []FolderBatchItemResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cutoff := time.Now().Add(-time.Duration(lastDays) * 24 * time.Hour)
	out := []FolderBatchItemResult{}
	for _, task := range s.tasks {
		for _, result := range task.Results {
			if result.Status == "failed" && result.Code != "" && result.Path != "" && (lastDays <= 0 || result.At.After(cutoff)) {
				out = append(out, result)
			}
		}
	}
	return out
}
