package service

import (
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
)

// ImportCastFromNFO 将本地 NFO 中的导演、演员导入数据库（演职人员关联表）。
// 仅在该媒体当前没有任何演职记录时执行，不覆盖在线刮削的结果。
// 返回导入的记录数。
func (s *MetadataService) ImportCastFromNFO(media *model.Media) int {
	if media == nil || media.ID == "" || media.FilePath == "" {
		return 0
	}
	if s.nfoService == nil || s.personRepo == nil || s.mediaPersonRepo == nil {
		return 0
	}
	if s.hasMediaCast(media.ID) {
		return 0
	}
	nfoPath := s.nfoService.FindNFOForMedia(media.FilePath)
	if nfoPath == "" {
		return 0
	}
	actors, directors, err := s.nfoService.GetActorsFromNFO(nfoPath)
	if err != nil {
		s.logger.Debugf("读取NFO演职人员失败: %s - %v", nfoPath, err)
		return 0
	}

	saved := 0
	seen := make(map[string]bool)
	add := func(name, role, character string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[role+"|"+name] {
			return
		}
		seen[role+"|"+name] = true
		person, err := s.personRepo.FindOrCreate(name, 0)
		if err != nil {
			return
		}
		mp := &model.MediaPerson{
			MediaID:   media.ID,
			PersonID:  person.ID,
			Role:      role,
			Character: strings.TrimSpace(character),
			SortOrder: saved,
		}
		if err := s.mediaPersonRepo.Create(mp); err == nil {
			saved++
		}
	}
	for _, d := range directors {
		add(d, "director", "")
	}
	for _, a := range actors {
		add(a.Name, "actor", a.Role)
	}
	if saved > 0 {
		s.logger.Debugf("从NFO导入 %d 条演职人员: %s", saved, media.Title)
	}
	return saved
}

// BackfillCastFromNFO 为库中尚无演职记录的电影补导入 NFO 演员，返回处理的影片数。
func (s *MetadataService) BackfillCastFromNFO() int {
	if s.mediaRepo == nil {
		return 0
	}
	list, err := s.mediaRepo.ListByMediaType("movie")
	if err != nil {
		s.logger.Warnf("NFO 演员回填：读取影片列表失败: %v", err)
		return 0
	}
	n := 0
	for i := range list {
		if s.ImportCastFromNFO(&list[i]) > 0 {
			n++
		}
	}
	if n > 0 {
		s.logger.Infof("NFO 演员回填完成：%d 部影片", n)
	}
	return n
}
