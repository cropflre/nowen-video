package model

import "gorm.io/gorm"

// AutoMigrateLite migrates only tables used by the NAS-oriented lite profile.
// Optional modules must opt in at startup before their persistence tables are
// created. Existing full-profile tables are left untouched, so deployments can
// switch between Lite and Full without destructive migrations.
func AutoMigrateLite(db *gorm.DB, enableAI bool) error {
	models := []any{
		&User{},
		&LoginLog{},
		&AuditLog{},
		&InviteCode{},
		&Library{},
		&SystemSetting{},
		&Series{},
		&Media{},
		&Person{},
		&MediaPerson{},
		&WatchHistory{},
		&Favorite{},
		&Playlist{},
		&PlaylistItem{},
		&Bookmark{},
		&ContentRating{},
		&UserPermission{},
		&PlaybackStats{},
		&ScrapeTask{},
		&ScrapeHistory{},
		&GenreMapping{},
		&RecommendCache{},
		&MovieCollection{},
		&SystemLog{},
		&FileOperationLog{},
	}

	if enableAI {
		models = append(models,
			&AICacheEntry{},
			&AIUsageRecord{},
			&AIFailoverLog{},
		)
	}

	if err := db.AutoMigrate(models...); err != nil {
		return err
	}

	// The safety net only adds columns to core tables that already exist. It
	// never creates tables belonging to disabled optional modules.
	ensureSQLiteColumns(db)
	return nil
}
