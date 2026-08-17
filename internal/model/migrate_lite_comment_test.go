package model

import "testing"

func TestAutoMigrateLiteIncludesCommentTable(t *testing.T) {
	db := openProfileDB(t, t.TempDir()+"/lite-comments.db")
	defer closeProfileDB(t, db)

	if err := AutoMigrateLite(db, false); err != nil {
		t.Fatalf("migrate lite profile: %v", err)
	}
	if !db.Migrator().HasTable(&Comment{}) {
		t.Fatal("lite profile must migrate comments table")
	}
}
