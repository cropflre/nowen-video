package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

func TestPathWithinRootDoesNotMatchSiblingPrefix(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "movie")
	child := filepath.Join(root, "season-1")
	sibling := filepath.Join(base, "movie-backup")

	if !pathWithinRoot(root, root) {
		t.Fatal("媒体库根目录应匹配自身")
	}
	if !pathWithinRoot(child, root) {
		t.Fatal("媒体库子目录应匹配根目录")
	}
	if pathWithinRoot(sibling, root) {
		t.Fatal("同前缀兄弟目录不能被误判为媒体库子目录")
	}
}

func TestFileWatcherUnwatchKeepsOtherLibraryRoots(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("创建 fsnotify watcher 失败: %v", err)
	}
	defer watcher.Close()

	base := t.TempDir()
	rootA := filepath.Join(base, "library-a")
	rootB := filepath.Join(base, "library-b")
	nestedA := filepath.Join(rootA, "nested")
	nestedB := filepath.Join(rootB, "nested")
	for _, dir := range []string{nestedA, nestedB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("创建测试目录失败: %v", err)
		}
	}

	registered := []string{rootA, nestedA, rootB, nestedB}
	for _, dir := range registered {
		if err := watcher.Add(dir); err != nil {
			t.Fatalf("注册 watcher 失败 %s: %v", dir, err)
		}
	}

	fw := &FileWatcherService{
		logger:  zap.NewNop().Sugar(),
		watcher: watcher,
		watching: map[string]string{
			rootA: "library-id",
			rootB: "library-id",
		},
		watchedDirs: map[string]map[string]struct{}{
			"library-id": {
				rootA:   {},
				nestedA: {},
				rootB:   {},
				nestedB: {},
			},
		},
		debounce: make(map[string]*time.Timer),
		stopCh:   make(chan struct{}),
	}

	fw.UnwatchLibrary(rootA)

	fw.mu.Lock()
	_, rootAStillWatching := fw.watching[rootA]
	_, rootBStillWatching := fw.watching[rootB]
	dirs := fw.watchedDirs["library-id"]
	_, hasRootA := dirs[rootA]
	_, hasNestedA := dirs[nestedA]
	_, hasRootB := dirs[rootB]
	_, hasNestedB := dirs[nestedB]
	fw.mu.Unlock()

	if rootAStillWatching || hasRootA || hasNestedA {
		t.Fatal("注销单个根目录后，不应保留该根目录的 watcher 索引")
	}
	if !rootBStillWatching || !hasRootB || !hasNestedB {
		t.Fatal("注销单个根目录不应影响同媒体库的其他根目录")
	}

	fw.UnwatchLibrary(rootB)
	fw.mu.Lock()
	_, hasLibraryDirs := fw.watchedDirs["library-id"]
	fw.mu.Unlock()
	if hasLibraryDirs {
		t.Fatal("最后一个根目录注销后应清空媒体库 watcher 索引")
	}
}
