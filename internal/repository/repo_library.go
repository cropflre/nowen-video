package repository

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== LibraryRepo ====================

type LibraryRepo struct {
	db *gorm.DB
}

func (r *LibraryRepo) Create(lib *model.Library) error {
	return r.db.Create(lib).Error
}

func (r *LibraryRepo) FindByID(id string) (*model.Library, error) {
	var lib model.Library
	err := r.db.First(&lib, "id = ?", id).Error
	return &lib, err
}

func (r *LibraryRepo) List() ([]model.Library, error) {
	var libs []model.Library
	err := r.db.Find(&libs).Error
	return libs, err
}

func (r *LibraryRepo) Update(lib *model.Library) error {
	return r.db.Omit("LastScanFailure").Save(lib).Error
}

func (r *LibraryRepo) ScanFailures() (map[string]string, error) {
	var libs []model.Library
	if err := r.db.Select("id", "last_scan_failure").Where("last_scan_failure <> ?", "").Find(&libs).Error; err != nil {
		return nil, err
	}
	failures := make(map[string]string, len(libs))
	for _, lib := range libs {
		failures[lib.ID] = lib.LastScanFailure
	}
	return failures, nil
}

// SetScanFailure updates only the scan result so a concurrent settings edit
// cannot overwrite a failure loaded from an older copy of the library.
func (r *LibraryRepo) SetScanFailure(id, message string) error {
	return r.db.Model(&model.Library{}).Where("id = ?", id).UpdateColumn("last_scan_failure", message).Error
}

func (r *LibraryRepo) Delete(id string) error {
	return r.db.Delete(&model.Library{}, "id = ?", id).Error
}
