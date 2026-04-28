// Package service 番号封面图处理（裁剪生成竖版 poster）
// 借鉴自 mdcx-master 的 img_crop.py
// 功能：
//   - Fanza/JavBus 等的封面是横版大图（含番号+标题+片段），需裁剪右侧 2/3 生成竖版
//   - 竖版 poster 更适合 Emby/Jellyfin 媒体墙展示
//   - 本实现使用 Go 标准库 image 包，零外部依赖
package service

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"  // 注册 PNG 解码
	_ "image/gif"  // 注册 GIF 解码
	"os"
	"path/filepath"
)

// CropPoster 将大图裁剪成竖版 poster
// 输入：原始封面路径（横版）
// 输出：竖版 poster 路径（同目录下 poster.jpg）
// 规则：以图片右侧为基准，裁剪出 宽:高 = 2:3 的区域（番号封面标准比例）
func CropPoster(coverPath string) (string, error) {
	if coverPath == "" {
		return "", fmt.Errorf("封面路径为空")
	}

	src, err := os.Open(coverPath)
	if err != nil {
		return "", fmt.Errorf("打开封面失败: %w", err)
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return "", fmt.Errorf("解码封面失败: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// 若图片已是竖版（宽 <= 高），直接复制
	if w <= h {
		return copyImageFile(coverPath, getPosterPath(coverPath))
	}

	// 计算裁剪区域：以右侧为基准，裁出 2:3 比例
	// 竖版比例 = 2:3，裁剪宽度 = h * 2 / 3
	cropW := h * 2 / 3
	if cropW > w {
		cropW = w
	}
	// 起点 X = 右边起算
	startX := w - cropW
	if startX < 0 {
		startX = 0
	}

	cropRect := image.Rect(startX, 0, w, h)

	// 裁剪（image.SubImage）
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	sub, ok := img.(subImager)
	if !ok {
		return "", fmt.Errorf("图片类型不支持裁剪: %T", img)
	}
	cropped := sub.SubImage(cropRect)

	// 保存
	posterPath := getPosterPath(coverPath)
	out, err := os.Create(posterPath)
	if err != nil {
		return "", fmt.Errorf("创建 poster 文件失败: %w", err)
	}
	defer out.Close()

	if err := jpeg.Encode(out, cropped, &jpeg.Options{Quality: 90}); err != nil {
		return "", fmt.Errorf("编码 poster 失败: %w", err)
	}
	return posterPath, nil
}

// GeneratePosterForMedia 为媒体文件生成 poster（竖版封面）
// 会在媒体同目录生成 <basename>-poster.jpg 和 poster.jpg 两个文件
// poster.jpg 供 Emby 识别，<basename>-poster.jpg 供 Kodi 识别
func (s *AdultScraperService) GeneratePosterForMedia(mediaFilePath, coverLocalPath string) (string, error) {
	if coverLocalPath == "" || mediaFilePath == "" {
		return "", nil
	}

	// 裁剪生成 poster
	posterPath, err := CropPoster(coverLocalPath)
	if err != nil {
		return "", err
	}

	// 额外生成 <basename>-poster.jpg（Kodi 风格）
	mediaDir := filepath.Dir(mediaFilePath)
	baseName := filepath.Base(mediaFilePath)
	ext := filepath.Ext(baseName)
	if ext != "" {
		baseName = baseName[:len(baseName)-len(ext)]
	}
	kodiPoster := filepath.Join(mediaDir, baseName+"-poster.jpg")
	if _, err := copyImageFile(posterPath, kodiPoster); err != nil {
		s.logger.Debugf("生成 Kodi poster 失败（不影响主流程）: %v", err)
	}

	return posterPath, nil
}

// ==================== 工具函数 ====================

// getPosterPath 基于 cover 路径推断 poster 路径
// /path/to/cover.jpg -> /path/to/poster.jpg
func getPosterPath(coverPath string) string {
	dir := filepath.Dir(coverPath)
	return filepath.Join(dir, "poster.jpg")
}

// copyImageFile 简单文件复制（与 subtitle_cleaner.go 的 copyFile 区分开）
func copyImageFile(src, dst string) (string, error) {
	input, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, input, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}
