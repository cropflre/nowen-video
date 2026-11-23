package service

import (
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
)

// normalizeNFOActorRole 将 NFO 中 <actor><type> 归一化为数据库角色值。
func normalizeNFOActorRole(actorType string) string {
	switch strings.ToLower(strings.TrimSpace(actorType)) {
	case "director":
		return "director"
	case "writer", "screenplay", "author":
		return "writer"
	default:
		return "actor"
	}
}

func (s *MetadataService) saveNFOCast(mediaID, seriesID string, actors []NFOActor, directors []string) int {
	saved := 0
	seen := make(map[string]bool)
	add := func(name, role, character string, tmdbID int) {
		name = strings.TrimSpace(name)
		if name == "" || seen[role+"|"+name] {
			return
		}
		seen[role+"|"+name] = true
		person, err := s.personRepo.FindOrCreate(name, tmdbID)
		if err != nil {
			return
		}
		if tmdbID > 0 && person.TMDbID == 0 {
			person.TMDbID = tmdbID
			_ = s.personRepo.Update(person)
		}
		mp := &model.MediaPerson{
			MediaID:   mediaID,
			SeriesID:  seriesID,
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
		add(d, "director", "", 0)
	}
	for _, a := range actors {
		role := normalizeNFOActorRole(a.Type)
		character := a.Role
		if role != "actor" {
			character = ""
		}
		add(a.Name, role, character, a.TMDbID)
	}
	return saved
}

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

	saved := s.saveNFOCast(media.ID, "", actors, directors)
	if saved > 0 {
		s.logger.Debugf("从NFO导入 %d 条演职人员: %s", saved, media.Title)
	}
	return saved
}

// ImportSeriesCastFromNFO 将剧集目录下的 tvshow.nfo 演职人员导入数据库。
// 仅在该剧集当前没有任何演职记录时执行，不覆盖在线刮削结果。
func (s *MetadataService) ImportSeriesCastFromNFO(series *model.Series) int {
	if series == nil || series.ID == "" || series.FolderPath == "" {
		return 0
	}
	if s.nfoService == nil || s.personRepo == nil || s.mediaPersonRepo == nil {
		return 0
	}
	if s.mediaPersonRepo.HasBySeriesID(series.ID) {
		return 0
	}
	nfoPath := s.nfoService.FindNFOFile(series.FolderPath)
	if nfoPath == "" {
		return 0
	}
	actors, directors, err := s.nfoService.GetActorsFromNFO(nfoPath)
	if err != nil {
		s.logger.Debugf("读取剧集NFO演职人员失败: %s - %v", nfoPath, err)
		return 0
	}

	saved := s.saveNFOCast("", series.ID, actors, directors)
	if saved > 0 {
		s.logger.Debugf("从剧集NFO导入 %d 条演职人员: %s", saved, series.Title)
	}
	return saved
}

// BackfillCastFromNFO 为库中尚无演职记录的电影和剧集补导入 NFO 演员，返回处理的条目数。
func (s *MetadataService) BackfillCastFromNFO() int {
	n := 0
	if s.mediaRepo != nil {
		list, err := s.mediaRepo.ListByMediaType("movie")
		if err != nil {
			s.logger.Warnf("NFO 演员回填：读取影片列表失败: %v", err)
		} else {
			for i := range list {
				if s.ImportCastFromNFO(&list[i]) > 0 {
					n++
				}
			}
		}
	}
	if s.seriesRepo != nil {
		seriesList, err := s.seriesRepo.ListAll("")
		if err != nil {
			s.logger.Warnf("NFO 演员回填：读取剧集列表失败: %v", err)
		} else {
			for i := range seriesList {
				if s.ImportSeriesCastFromNFO(&seriesList[i]) > 0 {
					n++
				}
			}
		}
	}
	if n > 0 {
		s.logger.Infof("NFO 演员回填完成：%d 部影视", n)
	}
	return n
}
