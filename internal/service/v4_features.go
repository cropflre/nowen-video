package service

import (
	"fmt"
)

// ==================== P1: 批量移动媒体服务 ====================

// BatchMoveMediaRequest 批量移动媒体请求
type BatchMoveMediaRequest struct {
	MediaIDs      []string `json:"media_ids"`
	TargetLibrary string   `json:"target_library_id"`
}

// BatchMoveResult 批量移动结果
type BatchMoveResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// BatchMoveMedia 批量移动媒体到目标媒体库
func (s *LibraryService) BatchMoveMedia(mediaIDs []string, targetLibraryID string) (*BatchMoveResult, error) {
	// 验证目标媒体库存在
	targetLib, err := s.repo.FindByID(targetLibraryID)
	if err != nil {
		return nil, fmt.Errorf("目标媒体库不存在")
	}

	result := &BatchMoveResult{Total: len(mediaIDs)}

	for _, mediaID := range mediaIDs {
		media, err := s.mediaRepo.FindByID(mediaID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("媒体 %s 不存在", mediaID))
			continue
		}

		// 跳过已在目标媒体库的
		if media.LibraryID == targetLibraryID {
			result.Success++
			continue
		}

		// 更新媒体库ID
		media.LibraryID = targetLibraryID
		if err := s.mediaRepo.Update(media); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("移动 %s 失败: %v", media.Title, err))
			continue
		}

		result.Success++
	}

	s.logger.Infof("批量移动媒体到 %s: 成功 %d, 失败 %d", targetLib.Name, result.Success, result.Failed)

	// 广播事件
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent("media_batch_moved", map[string]interface{}{
			"target_library": targetLib.Name,
			"success":        result.Success,
			"failed":         result.Failed,
		})
	}

	return result, nil
}
