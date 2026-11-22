package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// TestFindByTMDbIDColumn 实证 Person.TMDbID 的 GORM 派生列名，
// 并证明 FindByTMDbID 能按 TMDb ID 找到人。
//
// 证明方式不是推断命名规则：直接用 AutoMigrate 建表，再读
// PRAGMA table_info(persons) 看真实列名；随后写入一条带 TMDbID
// 的记录并用 FindByTMDbID 查回。
func TestFindByTMDbIDColumn(t *testing.T) {
	dsn := fmt.Sprintf("file:person-tmdb-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Person{}); err != nil {
		t.Fatal(err)
	}

	// 实证 1：读建表结果，确认 GORM 派生的真实列名。
	// 注意：Person 模型的表名是 GORM 复数化后的 people，不是 persons。
	type columnInfo struct {
		Name string `gorm:"column:name"`
	}
	var columns []columnInfo
	if err := db.Raw("PRAGMA table_info(people)").Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range columns {
		names[c.Name] = true
	}
	if !names["tm_db_id"] {
		t.Fatalf("persons 表缺少 GORM 派生列 tm_db_id，实际列=%v", names)
	}
	if names["tmdb_id"] {
		t.Fatalf("persons 表不应有 tmdb_id 列（那是死列名），实际列=%v", names)
	}

	// 实证 2：写入并按 TMDb ID 查回。
	repo := &PersonRepo{db: db}
	person := &model.Person{Name: "演员甲", TMDbID: 101}
	if err := repo.Create(person); err != nil {
		t.Fatal(err)
	}
	found, err := repo.FindByTMDbID(101)
	if err != nil {
		t.Fatalf("FindByTMDbID(101) 失败: %v", err)
	}
	if found.Name != "演员甲" || found.TMDbID != 101 {
		t.Fatalf("FindByTMDbID 返回了错误的人: %+v", found)
	}
	if _, err := repo.FindByTMDbID(999); err == nil {
		t.Fatal("FindByTMDbID(999) 应返回未找到错误")
	}
}
