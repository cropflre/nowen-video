package service

import (
	"errors"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupChildCatalog(t *testing.T) (*ChildCatalog, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	must := func(value interface{}) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	must(&model.User{ID: "child", Username: "child", Password: "hash", ChildMode: true})
	must(&model.User{ID: "parent", Username: "parent", Password: "hash", Role: "admin"})
	must(&model.Library{ID: "kids", Name: "Kids", Path: "/kids"})
	must(&model.Library{ID: "private", Name: "DO_NOT_LEAK_LIBRARY", Path: "/private"})
	must(&model.UserPermission{UserID: "child", AllowedLibraries: "kids", MaxRatingLevel: "G"})
	for _, id := range []string{"approved-show", "pending-show", "private-show"} {
		lib := "kids"
		if id == "private-show" {
			lib = "private"
		}
		must(&model.Series{ID: id, LibraryID: lib, Title: id, FolderPath: "/" + id})
	}
	rows := []struct{ id, library, rating, series string }{
		{"safe-a", "kids", "G", ""}, {"safe-b", "kids", "G", ""}, {"pg-only", "kids", "PG", ""},
		{"DO_NOT_LEAK_UNRATED", "kids", "", ""}, {"DO_NOT_LEAK_ADULT", "kids", "R", ""}, {"DO_NOT_LEAK_PRIVATE", "private", "G", ""},
		{"episode-1", "kids", "G", "approved-show"}, {"episode-2", "kids", "G", "approved-show"},
		{"pending-1", "kids", "G", "pending-show"}, {"DO_NOT_LEAK_EPISODE", "kids", "", "pending-show"},
		{"private-episode", "private", "G", "private-show"},
	}
	for i, row := range rows {
		kind := "movie"
		if row.series != "" {
			kind = "episode"
		}
		must(&model.Media{ID: row.id, Title: row.id, LibraryID: row.library, FilePath: "/" + row.id + ".mp4", MediaType: kind, SeriesID: row.series, SeasonNum: 1, EpisodeNum: i, Year: 2020 + i, CreatedAt: time.Now().Add(time.Duration(i) * time.Hour)})
		if row.rating != "" {
			must(&model.ContentRating{MediaID: row.id, Level: row.rating})
		}
		must(&model.Favorite{UserID: "child", MediaID: row.id})
		must(&model.WatchHistory{UserID: "child", MediaID: row.id, Position: 10, Duration: 100, Completed: false})
	}
	return NewChildCatalog(db), db
}
func TestChildCatalogFiltersBeforePaginationAndCounts(t *testing.T) {
	catalog, _ := setupChildCatalog(t)
	p, err := catalog.Policy("child")
	if err != nil {
		t.Fatal(err)
	}
	result, err := catalog.Mixed(p, ChildCatalogFilter{Sort: "title", Order: "asc"}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 || result.MovieCount != 2 || result.SeriesCount != 1 || len(result.Items) != 1 {
		t.Fatalf("unfiltered counts: %+v", result)
	}
	if result.Items[0].Type != "series" || result.Items[0].Series.ID != "approved-show" {
		t.Fatalf("ordering/pagination: %+v", result.Items)
	}
	movies, total, err := catalog.Media(p, ChildCatalogFilter{Type: "movie"}, 1, 1)
	if err != nil || total != 2 || len(movies) != 1 {
		t.Fatalf("media count %d len %d err %v", total, len(movies), err)
	}
	for _, filter := range []ChildCatalogFilter{{Query: "DO_NOT_LEAK"}, {LibraryID: "private"}, {YearMin: 2099}} {
		result, err = catalog.Mixed(p, filter, 1, 100)
		if err != nil || result.Total != 0 {
			t.Fatalf("restricted filter leaked: %+v %v", result, err)
		}
	}
}
func TestChildCatalogBlocksUnratedOtherLibrariesAndPartiallyReviewedShows(t *testing.T) {
	catalog, _ := setupChildCatalog(t)
	p, _ := catalog.Policy("child")
	for _, id := range []string{"DO_NOT_LEAK_UNRATED", "DO_NOT_LEAK_ADULT", "DO_NOT_LEAK_PRIVATE", "pg-only", "pending-1", "private-episode", "missing"} {
		if err := catalog.CheckMedia(p, id); err == nil {
			t.Errorf("allowed %s", id)
		}
	}
	if err := catalog.CheckMedia(p, "episode-1"); err != nil {
		t.Fatal(err)
	}
	if err := catalog.CheckSeries(p, "pending-show"); err == nil {
		t.Fatal("partially reviewed show exposed")
	}
	history, total, err := catalog.History(p, false, 1, 100)
	if err != nil || total != 4 || len(history) != 4 {
		t.Fatalf("history %d/%d %v", total, len(history), err)
	}
	favorites, total, err := catalog.Favorites(p, 1, 100)
	if err != nil || total != 4 || len(favorites) != 4 {
		t.Fatalf("favorites %d/%d %v", total, len(favorites), err)
	}
}
func TestChildCatalogNewEpisodeWithdrawsWholeShowUntilReviewed(t *testing.T) {
	catalog, db := setupChildCatalog(t)
	p, _ := catalog.Policy("child")
	if err := db.Create(&model.Media{ID: "new-episode", Title: "new", LibraryID: "kids", FilePath: "/new.mp4", MediaType: "episode", SeriesID: "approved-show", SeasonNum: 1, EpisodeNum: 99}).Error; err != nil {
		t.Fatal(err)
	}
	if catalog.CheckSeries(p, "approved-show") == nil || catalog.CheckMedia(p, "episode-1") == nil {
		t.Fatal("new unreviewed episode did not withdraw shared metadata")
	}
	if err := db.Create(&model.ContentRating{MediaID: "new-episode", Level: "G"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := catalog.CheckSeries(p, "approved-show"); err != nil {
		t.Fatal(err)
	}
}
func TestChildCatalogEmptyMissingOrInvalidPolicyDeniesByDefault(t *testing.T) {
	catalog, db := setupChildCatalog(t)
	if err := db.Where("user_id = ?", "child").Delete(&model.UserPermission{}).Error; err != nil {
		t.Fatal(err)
	}
	p, err := catalog.Policy("child")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.CheckMedia(p, "safe-a") == nil {
		t.Fatal("missing policy allowed media")
	}
	if err := db.Create(&model.UserPermission{UserID: "child", MaxRatingLevel: "G"}).Error; err != nil {
		t.Fatal(err)
	}
	p, _ = catalog.Policy("child")
	if catalog.CheckMedia(p, "safe-a") == nil {
		t.Fatal("empty library set allowed media")
	}
	if err := db.Model(&model.UserPermission{}).Where("user_id = ?", "child").Updates(map[string]interface{}{"allowed_libraries": "kids", "max_rating_level": "NC-17"}).Error; err != nil {
		t.Fatal(err)
	}
	p, _ = catalog.Policy("child")
	if catalog.CheckMedia(p, "safe-a") == nil {
		t.Fatal("invalid child rating allowed media")
	}
	if err := db.Migrator().DropTable(&model.ContentRating{}); err != nil {
		t.Fatal(err)
	}
	p.Ratings = []string{"G"}
	if catalog.CheckMedia(p, "safe-a") == nil {
		t.Fatal("storage failure granted access")
	}
}
func TestFamilyPolicyPreservesChildModeRevokesTokensAndRejectsAdmin(t *testing.T) {
	_, db := setupChildCatalog(t)
	repos := repository.NewRepositories(db)
	permissions := NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, zap.NewNop().Sugar(), db)
	if err := permissions.UpdateUserPermission("child", "kids", "PG", 0); err != nil {
		t.Fatal(err)
	}
	child, _ := repos.User.FindByID("child")
	if !child.ChildMode || child.TokenVersion != 1 {
		t.Fatalf("mode/token %+v", child)
	}
	if permissions.CheckContentRating("child", "DO_NOT_LEAK_UNRATED") {
		t.Fatal("unrated allowed")
	}
	if err := permissions.CheckMediaAccess("child", "DO_NOT_LEAK_PRIVATE", nil); err == nil {
		t.Fatal("direct access bypassed SQL scope")
	}
	if err := permissions.UpdateUserPermission("parent", "kids", "G", 0, true); !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("admin became child: %v", err)
	}
	parent, _ := repos.User.FindByID("parent")
	if parent.ChildMode || parent.TokenVersion != 0 {
		t.Fatal("failed policy partially committed")
	}
	if err := permissions.UpdateUserPermission("child", "kids", "G", 0, false); err != nil {
		t.Fatal(err)
	}
	child, _ = repos.User.FindByID("child")
	if child.ChildMode || child.TokenVersion != 2 {
		t.Fatal("mode withdrawal did not revoke prior tokens")
	}
}

func TestNewChildAccountStartsClosedAndCannotBecomeAdmin(t *testing.T) {
	catalog, db := setupChildCatalog(t)
	repos := repository.NewRepositories(db)
	users := NewUserService(repos.User, nil, nil, zap.NewNop().Sugar())
	created, err := users.CreateUser(&CreateUserRequest{Username: "new-child", Password: "child-password", ChildMode: true})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := repos.User.FindByID(created.ID)
	if err != nil || !loaded.ChildMode || loaded.Role != "user" || loaded.ViewingPreferences.AutoPlayNext == nil || *loaded.ViewingPreferences.AutoPlayNext {
		t.Fatalf("unsafe initial account: %+v, %v", loaded, err)
	}
	policy, err := catalog.Policy(created.ID)
	if err != nil || catalog.CheckMedia(policy, "safe-a") == nil {
		t.Fatalf("new account accessed media without parent setup: %v", err)
	}
	permissions := NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, zap.NewNop().Sugar(), db)
	permission, err := permissions.GetUserPermission(created.ID)
	if err != nil || !permission.ChildMode || permission.MaxRatingLevel != "G" || permission.AllowedLibraries != "" {
		t.Fatalf("unsafe permission defaults: %+v, %v", permission, err)
	}
	role := "admin"
	if _, err := users.UpdateUserByAdmin(created.ID, &UpdateUserRequest{Role: &role}); !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("child promoted to administrator: %v", err)
	}
	if _, err := users.CreateUser(&CreateUserRequest{Username: "invalid-child", Password: "child-password", ChildMode: true, Role: "admin"}); !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("administrator child account created: %v", err)
	}
}

func TestChildDailyLimitDoesNotTreatStorageFailureAsUnusedTime(t *testing.T) {
	_, db := setupChildCatalog(t)
	repos := repository.NewRepositories(db)
	permissions := NewPermissionService(repos.UserPermission, repos.ContentRating, repos.WatchHistory, zap.NewNop().Sugar(), db)
	if err := permissions.UpdateUserPermission("child", "kids", "G", 60); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.WatchHistory{}); err != nil {
		t.Fatal(err)
	}
	if ok, _, _ := permissions.CheckDailyTimeLimit("child"); ok {
		t.Fatal("storage failure treated as unused viewing time")
	}
}
