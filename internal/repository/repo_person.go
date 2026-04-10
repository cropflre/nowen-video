package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== PersonRepo ====================

type PersonRepo struct {
	db *gorm.DB
}

func (r *PersonRepo) Create(person *model.Person) error {
	return r.db.Create(person).Error
}

func (r *PersonRepo) Update(person *model.Person) error {
	return r.db.Save(person).Error
}

func (r *PersonRepo) FindByID(id string) (*model.Person, error) {
	var person model.Person
	err := r.db.First(&person, "id = ?", id).Error
	return &person, err
}

func (r *PersonRepo) FindByTMDbID(tmdbID int) (*model.Person, error) {
	var person model.Person
	err := r.db.Where("tmdb_id = ?", tmdbID).First(&person).Error
	return &person, err
}

func (r *PersonRepo) FindByName(name string) (*model.Person, error) {
	var person model.Person
	err := r.db.Where("name = ?", name).First(&person).Error
	return &person, err
}

func (r *PersonRepo) FindOrCreate(name string, tmdbID int) (*model.Person, error) {
	if tmdbID > 0 {
		person, err := r.FindByTMDbID(tmdbID)
		if err == nil {
			return person, nil
		}
	}
	person, err := r.FindByName(name)
	if err == nil {
		return person, nil
	}
	newPerson := &model.Person{Name: name, TMDbID: tmdbID}
	if err := r.Create(newPerson); err != nil {
		return nil, err
	}
	return newPerson, nil
}

func (r *PersonRepo) Search(keyword string, limit int) ([]model.Person, error) {
	var people []model.Person
	err := r.db.Where("name LIKE ?", "%"+keyword+"%").Limit(limit).Find(&people).Error
	return people, err
}

// ==================== MediaPersonRepo ====================

type MediaPersonRepo struct {
	db *gorm.DB
}

func (r *MediaPersonRepo) Create(mp *model.MediaPerson) error {
	return r.db.Create(mp).Error
}

func (r *MediaPersonRepo) ListByMediaID(mediaID string) ([]model.MediaPerson, error) {
	var mps []model.MediaPerson
	err := r.db.Preload("Person").Where("media_id = ?", mediaID).
		Order("role ASC, sort_order ASC").Find(&mps).Error
	return mps, err
}

func (r *MediaPersonRepo) ListBySeriesID(seriesID string) ([]model.MediaPerson, error) {
	var mps []model.MediaPerson
	err := r.db.Preload("Person").Where("series_id = ?", seriesID).
		Order("role ASC, sort_order ASC").Find(&mps).Error
	return mps, err
}

func (r *MediaPersonRepo) DeleteByMediaID(mediaID string) error {
	return r.db.Where("media_id = ?", mediaID).Delete(&model.MediaPerson{}).Error
}

func (r *MediaPersonRepo) DeleteBySeriesID(seriesID string) error {
	return r.db.Where("series_id = ?", seriesID).Delete(&model.MediaPerson{}).Error
}

// DeduplicateBySeriesID 清理同一 series_id 下重复的演职人员记录
// 相同 person_id + role 只保留 sort_order 最小的那条
func (r *MediaPersonRepo) DeduplicateBySeriesID(seriesID string) (int64, error) {
	// 子查询：找出每组 (person_id, role) 中需要保留的记录 ID（sort_order 最小的）
	// 然后删除不在保留列表中的记录
	result := r.db.Exec(`
		DELETE FROM media_persons
		WHERE series_id = ? AND id NOT IN (
			SELECT keep_id FROM (
				SELECT MIN(id) as keep_id
				FROM media_persons
				WHERE series_id = ?
				GROUP BY person_id, role
			) AS keeper
		)
	`, seriesID, seriesID)
	return result.RowsAffected, result.Error
}

func (r *MediaPersonRepo) ListByPersonID(personID string) ([]model.MediaPerson, error) {
	var mps []model.MediaPerson
	err := r.db.Where("person_id = ?", personID).Find(&mps).Error
	return mps, err
}

// ListMediaByPersonID 根据 person_id 查询该演员参演的所有影视作品（电影）
func (r *MediaPersonRepo) ListMediaByPersonID(personID string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.
		Where("id IN (?)",
			r.db.Model(&model.MediaPerson{}).Select("media_id").Where("person_id = ? AND media_id != ''", personID),
		).
		Order("year DESC, created_at DESC").
		Find(&media).Error
	return media, err
}

// ListSeriesByPersonID 根据 person_id 查询该演员参演的所有剧集合集
func (r *MediaPersonRepo) ListSeriesByPersonID(personID string) ([]model.Series, error) {
	var series []model.Series
	err := r.db.
		Where("id IN (?)",
			r.db.Model(&model.MediaPerson{}).Select("series_id").Where("person_id = ? AND series_id != ''", personID),
		).
		Order("year DESC, created_at DESC").
		Find(&series).Error
	return series, err
}
