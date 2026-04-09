import api from './client'
import type {
  BatchMoveRequest,
  BatchMoveResult,
} from '@/types'

// ==================== P1: 批量移动媒体 ====================
export const batchMoveApi = {
  batchMove: (data: BatchMoveRequest) =>
    api.post<{ data: BatchMoveResult }>('/admin/media/batch-move', data),
}
