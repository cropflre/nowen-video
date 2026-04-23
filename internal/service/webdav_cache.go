package service

import (
	"container/list"
	"fmt"
	"io"
	"sync"
)

// ==================== WebDAV 自适应块缓存（LRU） ====================
//
// 背景：每次 player seek 都触发一次独立的 HTTP Range 请求，往返延迟叠加。
// 方案：按固定块大小（8MB）缓存最近访问的数据块，相邻区间的读取可直接命中缓存。
//
// 典型命中场景：
//   - 播放器 demuxer 连续读取多个相邻小块（MP4 box、MKV cluster）
//   - 音频/视频/字幕轨道交织在同一时间窗口
//   - seek 后的短距离"回看/前探"
//
// 参数选择：
//   - BlockSize = 8MB：足够覆盖一个 HLS 分片常见大小，也不会浪费太多带宽
//   - MaxBlocks = 4：每个文件最多占用 32MB 内存，支持中等规模并发
//   - 上层调用者可通过 WebDAVCacheBlockSize/WebDAVCacheBlockCount 调整（见 config）

const (
	defaultWebDAVBlockSize  int64 = 8 * 1024 * 1024 // 8 MiB
	defaultWebDAVBlockCount       = 4               // 每文件最多 4 块
)

// webdavCacheBlock 一个缓存块
type webdavCacheBlock struct {
	offset int64  // 块起始偏移（按 blockSize 对齐）
	data   []byte // 真实数据（长度可能小于 blockSize，最后一块不满时）
}

// webdavBlockCache 单文件的 LRU 块缓存
//
// 所有方法线程安全，使用 container/list 实现 O(1) LRU。
type webdavBlockCache struct {
	mu         sync.Mutex
	blockSize  int64
	maxBlocks  int
	lru        *list.List              // 双向链表：front=最近使用
	index      map[int64]*list.Element // offset → 链表节点
	fetchGroup map[int64]chan struct{} // 防止同一块并发发起多次 Range
	fetchMu    sync.Mutex
}

// newWebDAVBlockCache 创建块缓存
func newWebDAVBlockCache(blockSize int64, maxBlocks int) *webdavBlockCache {
	if blockSize <= 0 {
		blockSize = defaultWebDAVBlockSize
	}
	if maxBlocks <= 0 {
		maxBlocks = defaultWebDAVBlockCount
	}
	return &webdavBlockCache{
		blockSize:  blockSize,
		maxBlocks:  maxBlocks,
		lru:        list.New(),
		index:      make(map[int64]*list.Element),
		fetchGroup: make(map[int64]chan struct{}),
	}
}

// get 查询缓存块；命中返回数据（可直接使用，调用方无需复制）
func (c *webdavBlockCache) get(offset int64) (*webdavCacheBlock, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.index[offset]; ok {
		c.lru.MoveToFront(el)
		return el.Value.(*webdavCacheBlock), true
	}
	return nil, false
}

// put 放入缓存块（淘汰最旧块）
func (c *webdavBlockCache) put(block *webdavCacheBlock) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.index[block.offset]; ok {
		el.Value = block
		c.lru.MoveToFront(el)
		return
	}

	el := c.lru.PushFront(block)
	c.index[block.offset] = el

	// 淘汰最旧
	for c.lru.Len() > c.maxBlocks {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.lru.Remove(oldest)
		b := oldest.Value.(*webdavCacheBlock)
		delete(c.index, b.offset)
	}
}

// clear 清空缓存（例如文件关闭时）
func (c *webdavBlockCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Init()
	c.index = make(map[int64]*list.Element)
}

// acquireFetch 获取某块的"独占获取权"；避免多 goroutine 对同一块发起重复 Range
// 返回值：
//   - wait == nil: 当前 goroutine 负责下载，下载完成后必须调用 releaseFetch
//   - wait != nil: 已有其他 goroutine 在下载，读取者应 wait 然后重查缓存
func (c *webdavBlockCache) acquireFetch(offset int64) (wait <-chan struct{}) {
	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()

	if ch, ok := c.fetchGroup[offset]; ok {
		return ch
	}
	ch := make(chan struct{})
	c.fetchGroup[offset] = ch
	return nil
}

// releaseFetch 释放获取权并唤醒所有等待者
func (c *webdavBlockCache) releaseFetch(offset int64) {
	c.fetchMu.Lock()
	ch, ok := c.fetchGroup[offset]
	if ok {
		delete(c.fetchGroup, offset)
	}
	c.fetchMu.Unlock()
	if ok {
		close(ch)
	}
}

// ==================== 缓存化 ReadAt ====================

// readAtCached 使用块缓存的 ReadAt 实现
//
// 算法：
//  1. 将请求区间 [off, off+len) 切成按 blockSize 对齐的若干块
//  2. 每块尝试从缓存取；未命中则发起 Range 获取整块并放入缓存
//  3. 从缓存块中按需截取、拷贝到 p
//
// 注意：非并发关键路径，锁粒度较粗以保持实现简洁。
func (f *webdavFile) readAtCached(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off < 0 {
		return 0, fmt.Errorf("ReadAt: negative offset")
	}

	cache := f.blockCache
	if cache == nil {
		// 未启用缓存，走直连 Range
		return f.readAtDirect(p, off)
	}

	// 文件总大小（用于判断 EOF）
	fileSize := int64(-1)
	if f.stat != nil {
		fileSize = f.stat.Size()
	}

	blockSize := cache.blockSize
	total := 0
	cur := off
	end := off + int64(len(p))

	for cur < end {
		// 对齐到块起始
		blockOff := (cur / blockSize) * blockSize

		// 读取或下载该块
		block, err := f.ensureBlock(cache, blockOff, fileSize)
		if err != nil {
			if total > 0 {
				// 已读到一部分，如果是 EOF 保留 total
				if err == io.EOF {
					return total, io.EOF
				}
				return total, err
			}
			return 0, err
		}

		// 在块中的偏移
		inBlock := cur - block.offset
		if inBlock >= int64(len(block.data)) {
			// 该块数据不足（EOF 块）
			if total > 0 {
				return total, io.EOF
			}
			return 0, io.EOF
		}

		// 拷贝到 p
		n := copy(p[total:], block.data[inBlock:])
		total += n
		cur += int64(n)

		// 如果本块已到 EOF 且未读满 p，返回 EOF
		if int64(len(block.data)) < blockSize && cur >= block.offset+int64(len(block.data)) {
			if total < len(p) {
				return total, io.EOF
			}
		}

		if n == 0 {
			break
		}
	}

	if total < len(p) {
		return total, io.EOF
	}
	return total, nil
}

// ensureBlock 保证某块存在于缓存（优先命中，其次下载）
func (f *webdavFile) ensureBlock(cache *webdavBlockCache, blockOff int64, fileSize int64) (*webdavCacheBlock, error) {
	// 1) 快速查询
	if b, ok := cache.get(blockOff); ok {
		return b, nil
	}

	// 2) 协调并发下载：同一块只允许一个 goroutine 执行 HTTP Range
	if wait := cache.acquireFetch(blockOff); wait != nil {
		<-wait
		// 被唤醒后重新查缓存
		if b, ok := cache.get(blockOff); ok {
			return b, nil
		}
		// 下载方可能失败了，fallthrough 让当前 goroutine 自行重试一次
	}
	defer cache.releaseFetch(blockOff)

	// 3) 计算实际下载长度
	readLen := cache.blockSize
	if fileSize >= 0 {
		if blockOff >= fileSize {
			return nil, io.EOF
		}
		if blockOff+readLen > fileSize {
			readLen = fileSize - blockOff
		}
	}

	// 4) 发起 Range 请求
	data := make([]byte, readLen)
	n, err := f.readAtDirect(data, blockOff)
	if err != nil && err != io.EOF {
		return nil, err
	}
	block := &webdavCacheBlock{
		offset: blockOff,
		data:   data[:n],
	}
	cache.put(block)
	return block, nil
}
