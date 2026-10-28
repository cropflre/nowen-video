package service

import (
	"context"
	"errors"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestAgentManagementInspectionRequiresCurrentAdministratorAndReportsRealCounts(t *testing.T) {
	db, agent := agentFixture(t)
	if err := db.Create(&model.User{ID: "owner", Username: "owner", Role: "admin", Password: "hash"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{"child", "adult", "other"} {
		out, err := agent.Chat(context.Background(), uid, ViewerAgentRequest{Message: "检查影视库"})
		if !errors.Is(err, ErrForbidden) || out.Management != nil {
			t.Fatalf("%s read administrative state: %+v %v", uid, out, err)
		}
	}
	out, err := agent.Chat(context.Background(), "owner", ViewerAgentRequest{Message: "检查影视库"})
	if err != nil || out.Management == nil {
		t.Fatalf("inspection: %+v %v", out, err)
	}
	data := out.Management
	if data.LibraryCount != 2 || data.MediaCount != 8 || data.NeedsRating != 1 || data.NeedsMetadata != 8 {
		t.Fatalf("incorrect administrative counts: %+v", data)
	}
	var sum int64
	for _, library := range data.Libraries {
		sum += library.MediaCount
	}
	if sum != 8 {
		t.Fatalf("per-library counts disagree: %+v", data.Libraries)
	}
	out, err = agent.Chat(context.Background(), "owner", ViewerAgentRequest{Message: "检查任务"})
	if err != nil || out.Action == nil || out.Action.Kind != "inspect_tasks" {
		t.Fatalf("task read action: %+v %v", out, err)
	}
	if err = db.Model(&model.User{}).Where("id = ?", "owner").Update("role", "user").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = agent.Chat(context.Background(), "owner", ViewerAgentRequest{Message: "检查任务"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("demoted user retained management authority: %v", err)
	}
	var mediaCount int64
	db.Model(&model.Media{}).Count(&mediaCount)
	if mediaCount != 8 {
		t.Fatal("read-only inspection changed the library")
	}
}
