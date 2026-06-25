package game

import (
	"testing"

	"werewolf/backend/internal/config"
)

func TestCreateHumanGamePausesForHumanDecision(t *testing.T) {
	service := NewService(testPresets())
	snapshot, err := service.CreateGame(CreateRequest{
		TemplateID:    "classic-6",
		SpectatorMode: false,
		HumanName:     "测试玩家",
		AIPresetIDs:   []string{"steady-reader", "steady-reader", "steady-reader", "steady-reader", "steady-reader"},
		Seed:          1,
	})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	if snapshot.Mode != "human" {
		t.Fatalf("Mode = %q, want human", snapshot.Mode)
	}
	if snapshot.PendingAction == nil {
		t.Fatalf("PendingAction = nil, want human-visible decision")
	}
	if snapshot.HeroState == nil || snapshot.HeroState.Role == "" {
		t.Fatalf("HeroState missing role info: %+v", snapshot.HeroState)
	}
}

func TestSpectatorGameCanAutoFinish(t *testing.T) {
	service := NewService(testPresets())
	snapshot, err := service.CreateGame(CreateRequest{
		TemplateID:    "classic-9",
		SpectatorMode: true,
		AIPresetIDs: []string{
			"steady-reader", "pressure-caller", "chaos-engine",
			"steady-reader", "pressure-caller", "chaos-engine",
			"steady-reader", "pressure-caller", "chaos-engine",
		},
		Seed: 7,
	})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	finished, err := service.Act(snapshot.ID, ActionRequest{Action: "auto_finish"})
	if err != nil {
		t.Fatalf("Act(auto_finish) error = %v", err)
	}
	if finished.Status != "finished" {
		t.Fatalf("Status = %q, want finished", finished.Status)
	}
	if finished.WinnerSide == "" {
		t.Fatalf("WinnerSide empty after auto finish")
	}
	replay, ok := service.GetReplay(snapshot.ID)
	if !ok {
		t.Fatalf("GetReplay(%q) = false, want true", snapshot.ID)
	}
	if len(replay.Events) == 0 {
		t.Fatalf("Replay has no events")
	}
}

func testPresets() []config.Preset {
	return []config.Preset{
		{ID: "steady-reader", Name: "稳健派", Style: "steady", Persona: "test"},
		{ID: "pressure-caller", Name: "施压派", Style: "pressure", Persona: "test"},
		{ID: "chaos-engine", Name: "搅局派", Style: "chaos", Persona: "test"},
	}
}
