package service

import (
	"fmt"
	"os"

	"github.com/nowen-video/nowen-video/internal/model"
)

// ScanNewMovies is synchronous so a durable caller can record the actual result.
// It shares the ordinary library scan lock but skips every post-processing stage.
func (s *LibraryService) ScanNewMovies(lib *model.Library) (int, error) {
	if lib.Type != "movie" && lib.Type != "other" {
		return 0, fmt.Errorf("暂仅支持电影库")
	}
	if _, busy := s.scanning.LoadOrStore(lib.ID, true); busy {
		return 0, ErrScanInProgress
	}
	defer s.scanning.Delete(lib.ID)
	defer s.clearScanPhaseState(lib.ID)
	s.setScanPhaseState(&ScanPhaseData{LibraryID: lib.ID, LibraryName: lib.Name, Phase: "scanning", StepCurrent: 1, StepTotal: 1, Message: "正在查找并登记新电影"})
	if len(lib.AllPaths()) == 0 {
		return 0, fmt.Errorf("未配置媒体文件夹")
	}
	for _, path := range lib.AllPaths() {
		info, err := os.Stat(path)
		if err != nil {
			return 0, err
		}
		if !info.IsDir() {
			return 0, fmt.Errorf("媒体路径不是文件夹")
		}
	}
	return s.scanner.ScanLibraryWithOptions(lib, ScanLibraryOptions{AddOnly: true, SuppressCompletionCallback: true})
}
