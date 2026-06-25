package game

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	backendai "werewolf/backend/internal/ai"
	"werewolf/backend/internal/config"
)

type Service struct {
	mu             sync.RWMutex
	templates      []Template
	templateByID   map[string]Template
	presets        []config.Preset
	presetByID     map[string]config.Preset
	aiClient       *backendai.Client
	games          map[string]*gameState
	subscribers    map[string]map[*subscriber]struct{}
	autoplaying    map[string]bool
}

type subscriber struct {
	ch             chan Event
	includePrivate bool
}

type playerState struct {
	Seat         int
	Name         string
	Alive        bool
	IsHuman      bool
	Preset       config.Preset
	Role         Role
	RevealedRole string
	LastSpeech   string
	LastVote     int
}

type gameState struct {
	ID               string
	Mode             string
	Template         Template
	Day              int
	Phase            string
	Status           string
	Message          string
	Control          ControlState
	Players          []playerState
	HeroSeat         int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	FinishedAt       time.Time
	Sequence         int
	Events           []Event
	PhaseActors      []int
	PhaseIndex       int
	WinnerSide       string
	PendingResume    string
	PendingNightKill int
	NightHealTarget  int
	NightPoisonTarget int
	NightGuardTarget int
	NightWolfVotes   map[int]int
	SeerFindings     map[int]Role
	SeerChecked      map[int]struct{}
	WitchHealUsed    bool
	WitchPoisonUsed  bool
	GuardLastTarget  int
	HunterShotUsed   bool
	RNG              *rand.Rand
}

func NewService(presets []config.Preset) *Service {
	templates := DefaultTemplates()
	templateByID := make(map[string]Template, len(templates))
	for _, template := range templates {
		templateByID[template.ID] = template
	}
	presetByID := make(map[string]config.Preset, len(presets))
	for _, preset := range presets {
		presetByID[preset.ID] = preset
	}
	return &Service{
		templates:    templates,
		templateByID: templateByID,
		presets:      append([]config.Preset(nil), presets...),
		presetByID:   presetByID,
		aiClient:     backendai.NewClient(),
		games:        map[string]*gameState{},
		subscribers:  map[string]map[*subscriber]struct{}{},
		autoplaying:  map[string]bool{},
	}
}

func (s *Service) ListTemplates() []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Template, len(s.templates))
	copy(out, s.templates)
	return out
}

func (s *Service) ListPresets() []config.Preset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]config.Preset, len(s.presets))
	copy(out, s.presets)
	return out
}

func (s *Service) CreateGame(req CreateRequest) (Snapshot, error) {
	s.mu.Lock()

	template, ok := s.templateByID[strings.TrimSpace(req.TemplateID)]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("unknown template %q", req.TemplateID)
	}
	requiredAI := template.Seats
	mode := "spectator"
	if !req.SpectatorMode {
		mode = "human"
		requiredAI--
	}
	if len(req.AIPresetIDs) != requiredAI {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("template %s needs %d AI presets", template.Name, requiredAI)
	}
	players := make([]playerState, 0, template.Seats)
	if !req.SpectatorMode {
		humanName := strings.TrimSpace(req.HumanName)
		if humanName == "" {
			humanName = "你"
		}
		players = append(players, playerState{Seat: 0, Name: humanName, Alive: true, IsHuman: true, LastVote: -1})
	}
	usedNames := map[string]int{}
	for idx, presetID := range req.AIPresetIDs {
		preset, ok := s.presetByID[presetID]
		if !ok {
			s.mu.Unlock()
			return Snapshot{}, fmt.Errorf("unknown preset %q", presetID)
		}
		seat := idx
		if !req.SpectatorMode {
			seat++
		}
		name := preset.Name
		usedNames[name]++
		if usedNames[name] > 1 {
			name = fmt.Sprintf("%s #%d", preset.Name, usedNames[name])
		}
		players = append(players, playerState{Seat: seat, Name: name, Alive: true, Preset: preset, LastVote: -1})
	}
	sort.Slice(players, func(i, j int) bool { return players[i].Seat < players[j].Seat })

	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))
	roles := append([]Role(nil), template.Roles...)
	rng.Shuffle(len(roles), func(i, j int) {
		roles[i], roles[j] = roles[j], roles[i]
	})
	for idx := range players {
		players[idx].Role = roles[idx]
	}

	id, err := newID()
	if err != nil {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("generate id: %w", err)
	}
	semiAuto := req.SpectatorMode && !req.ManualMode && req.SemiAutoMode
	manual := req.SpectatorMode && req.ManualMode
	now := time.Now().UTC()
	game := &gameState{
		ID:                id,
		Mode:              mode,
		Template:          template,
		Day:               1,
		Status:            "running",
		Control: ControlState{
			SpectatorMode: req.SpectatorMode,
			SemiAutoMode:  semiAuto,
			ManualMode:    manual,
			CanStep:       false,
			Running:       req.SpectatorMode && !manual,
		},
		Players:           players,
		HeroSeat:          0,
		CreatedAt:         now,
		UpdatedAt:         now,
		PendingNightKill:  -1,
		NightHealTarget:   -1,
		NightPoisonTarget: -1,
		NightGuardTarget:  -1,
		GuardLastTarget:   -1,
		NightWolfVotes:    map[int]int{},
		SeerFindings:      map[int]Role{},
		SeerChecked:       map[int]struct{}{},
		RNG:               rng,
	}
	if req.SpectatorMode {
		game.HeroSeat = -1
	}
	s.games[id] = game
	s.appendEvent(game, "game_created", "public", map[string]any{
		"template": template.Name,
		"mode":     mode,
		"players":  playerNames(game.Players),
	})
	s.startNight(game)
	oldSeq := 0
	snapshot := s.snapshotLocked(game)
	newEvents := visibleNewEvents(game, oldSeq, game.Mode == "spectator")
	s.mu.Unlock()
	s.publish(id, newEvents)
	if game.Mode == "spectator" {
		if manual {
			return snapshot, nil
		}
		s.startAutoplayIfNeeded(id)
		updated, _ := s.GetGame(id)
		return updated, nil
	}
	return s.runHumanUntilPause(id)
}

func (s *Service) GetGame(id string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	game, ok := s.games[id]
	if !ok {
		return Snapshot{}, false
	}
	return s.snapshotLocked(game), true
}

func (s *Service) startAutoplayIfNeeded(id string) {
	s.mu.Lock()
	if s.autoplaying[id] {
		s.mu.Unlock()
		return
	}
	game, ok := s.games[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	s.autoplaying[id] = true
	game.Control.Running = true
	s.mu.Unlock()
	go s.runAutoplay(id)
}

func (s *Service) finishAutoplay(id string) {
	s.mu.Lock()
	delete(s.autoplaying, id)
	if game, ok := s.games[id]; ok {
		game.Control.Running = false
	}
	s.mu.Unlock()
}

func (s *Service) runAutoplay(id string) {
	defer s.finishAutoplay(id)
	for {
		snapshot, events, done, err := s.runAutoplayIteration(id)
		_ = snapshot
		if len(events) > 0 {
			s.publish(id, events)
		}
		if err != nil || done {
			return
		}
	}
}

func (s *Service) runAutoplayIteration(id string) (Snapshot, []Event, bool, error) {
	s.mu.Lock()
	game, ok := s.games[id]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, nil, true, fmt.Errorf("game not found")
	}
	oldSeq := game.Sequence
	if game.Control.Stopped {
		game.Status = "stopped"
		snapshot := s.snapshotLocked(game)
		events := visibleNewEvents(game, oldSeq, game.Mode == "spectator")
		s.mu.Unlock()
		return snapshot, events, true, nil
	}
	if game.Status != "running" {
		snapshot := s.snapshotLocked(game)
		s.mu.Unlock()
		return snapshot, nil, true, nil
	}
	prevDay := game.Day
	actor := s.currentActor(game)
	if actor < 0 && game.PendingResume != "" {
		s.resumePendingState(game)
		actor = s.currentActor(game)
	}
	if actor < 0 {
		snapshot := s.snapshotLocked(game)
		s.mu.Unlock()
		return snapshot, nil, true, nil
	}
	if game.Mode == "spectator" {
		if game.Control.Paused {
			snapshot := s.snapshotLocked(game)
			s.mu.Unlock()
			return snapshot, nil, true, nil
		}
		if game.Control.ManualMode {
			if !game.Control.CanStep {
				snapshot := s.snapshotLocked(game)
				s.mu.Unlock()
				return snapshot, nil, true, nil
			}
			game.Control.CanStep = false
		}
	}
	if game.Players[actor].IsHuman {
		snapshot := s.snapshotLocked(game)
		s.mu.Unlock()
		return snapshot, nil, true, nil
	}
	s.applyAIAction(game, actor)
	snapshot := s.snapshotLocked(game)
	events := visibleNewEvents(game, oldSeq, game.Mode == "spectator")
	done := false
	if game.Status != "running" {
		done = true
	} else if game.Mode == "spectator" {
		if game.Control.ManualMode {
			done = true
		} else if game.Control.SemiAutoMode {
			if !game.Control.Running || game.Day > prevDay {
				done = true
			}
		}
	}
	s.mu.Unlock()
	return snapshot, events, done, nil
}

func (s *Service) runHumanUntilPause(id string) (Snapshot, error) {
	s.mu.Lock()
	game, ok := s.games[id]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("game not found")
	}
	oldSeq := game.Sequence
	for game.Status == "running" {
		actor := s.currentActor(game)
		if actor < 0 || game.Players[actor].IsHuman {
			break
		}
		s.applyAIAction(game, actor)
	}
	snapshot := s.snapshotLocked(game)
	events := visibleNewEvents(game, oldSeq, game.Mode == "spectator")
	s.mu.Unlock()
	s.publish(id, events)
	return snapshot, nil
}

func (s *Service) Act(id string, req ActionRequest) (Snapshot, error) {
	s.mu.Lock()
	game, ok := s.games[id]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("game not found")
	}
	if game.Status != "running" {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("game already finished")
	}
	oldSeq := game.Sequence
	if err := s.actLocked(game, req); err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}
	snapshot := s.snapshotLocked(game)
	newEvents := visibleNewEvents(game, oldSeq, game.Mode == "spectator")
	s.mu.Unlock()
	s.publish(id, newEvents)
	return snapshot, nil
}

func (s *Service) ControlGame(id string, req ControlRequest) (Snapshot, error) {
	s.mu.Lock()
	game, ok := s.games[id]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("game not found")
	}
	switch req.Action {
	case "pause":
		game.Control.Paused = true
	case "continue":
		game.Control.Paused = false
		if game.Control.SpectatorMode && game.Control.ManualMode {
			game.Control.CanStep = true
		}
	case "auto_on":
		game.Control.SemiAutoMode = false
		game.Control.ManualMode = false
		game.Control.Paused = false
		game.Control.CanStep = false
	case "semi_auto_on":
		game.Control.SemiAutoMode = true
		game.Control.ManualMode = false
		game.Control.Paused = false
		game.Control.CanStep = false
	case "manual_on":
		game.Control.ManualMode = true
		game.Control.SemiAutoMode = false
		game.Control.Paused = false
		game.Control.CanStep = false
	case "step":
		game.Control.ManualMode = true
		game.Control.SemiAutoMode = false
		game.Control.Paused = false
		game.Control.CanStep = true
	case "stop":
		game.Control.Stopped = true
		game.Status = "stopped"
	default:
		s.mu.Unlock()
		return Snapshot{}, fmt.Errorf("unsupported control action %q", req.Action)
	}
	snapshot := s.snapshotLocked(game)
	s.mu.Unlock()
	if req.Action == "continue" || req.Action == "auto_on" || req.Action == "semi_auto_on" || req.Action == "step" {
		s.startAutoplayIfNeeded(id)
		updated, _ := s.GetGame(id)
		return updated, nil
	}
	return snapshot, nil
}

func (s *Service) ListRecords() []RecordSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RecordSummary, 0, len(s.games))
	for _, game := range s.games {
		out = append(out, s.recordSummaryLocked(game))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *Service) GetReplay(id string) (ReplayDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	game, ok := s.games[id]
	if !ok {
		return ReplayDetail{}, false
	}
	players := make([]ReplayPlayer, len(game.Players))
	for idx, player := range game.Players {
		players[idx] = ReplayPlayer{
			Seat:     player.Seat,
			Name:     player.Name,
			Role:     string(player.Role),
			Alive:    player.Alive,
			IsHuman:  player.IsHuman,
			PresetID: player.Preset.ID,
		}
	}
	return ReplayDetail{
		Summary:   s.recordSummaryLocked(game),
		Players:   players,
		Events:    append([]Event(nil), game.Events...),
		Template:  game.Template,
		CreatedAt: game.CreatedAt,
	}, true
}

func (s *Service) Subscribe(id string, includePrivate bool) (<-chan Event, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.games[id]; !ok {
		return nil, nil, fmt.Errorf("game not found")
	}
	sub := &subscriber{ch: make(chan Event, 32), includePrivate: includePrivate}
	if s.subscribers[id] == nil {
		s.subscribers[id] = map[*subscriber]struct{}{}
	}
	s.subscribers[id][sub] = struct{}{}
	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subs, ok := s.subscribers[id]; ok {
			if _, exists := subs[sub]; exists {
				delete(subs, sub)
				close(sub.ch)
			}
			if len(subs) == 0 {
				delete(s.subscribers, id)
			}
		}
	}
	return sub.ch, unsubscribe, nil
}

func (s *Service) publish(id string, events []Event) {
	if len(events) == 0 {
		return
	}
	s.mu.RLock()
	subs := make([]*subscriber, 0, len(s.subscribers[id]))
	for sub := range s.subscribers[id] {
		subs = append(subs, sub)
	}
	s.mu.RUnlock()
	for _, event := range events {
		for _, sub := range subs {
			if !sub.includePrivate && event.Visibility != "public" {
				continue
			}
			select {
			case sub.ch <- event:
			default:
			}
		}
	}
}

func (s *Service) actLocked(game *gameState, req ActionRequest) error {
	action := strings.TrimSpace(req.Action)
	switch action {
	case "continue":
		if game.Mode != "spectator" {
			return fmt.Errorf("continue only available in spectator mode")
		}
		return s.runOneStep(game)
	case "auto_finish":
		if game.Mode != "spectator" {
			return fmt.Errorf("auto_finish only available in spectator mode")
		}
		for game.Status == "running" {
			if err := s.runOneStep(game); err != nil {
				return err
			}
		}
		return nil
	case "speech":
		return s.submitHumanSpeech(game, req.Text)
	case "select":
		return s.submitHumanTarget(game, req.TargetSeat)
	case "witch":
		return s.submitHumanWitch(game, req)
	case "pass":
		return s.submitHumanPass(game)
	default:
		return fmt.Errorf("unsupported action %q", req.Action)
	}
}

func (s *Service) submitHumanSpeech(game *gameState, text string) error {
	actor := s.currentActor(game)
	if actor < 0 || !game.Players[actor].IsHuman {
		return fmt.Errorf("当前不是你的发言回合")
	}
	if game.Phase != "day_main" && game.Phase != "day_reply" {
		return fmt.Errorf("当前阶段不能发言")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("发言不能为空")
	}
	s.applySpeech(game, actor, text)
	s.runAIUntilPause(game)
	return nil
}

func (s *Service) submitHumanTarget(game *gameState, target *int) error {
	actor := s.currentActor(game)
	if actor < 0 || !game.Players[actor].IsHuman {
		return fmt.Errorf("当前不是你的行动回合")
	}
	if target == nil {
		return fmt.Errorf("缺少目标")
	}
	if !containsInt(s.allowedTargets(game, actor), *target) {
		return fmt.Errorf("目标不合法")
	}
	s.applyTargetAction(game, actor, *target)
	s.runAIUntilPause(game)
	return nil
}

func (s *Service) submitHumanWitch(game *gameState, req ActionRequest) error {
	actor := s.currentActor(game)
	if actor < 0 || !game.Players[actor].IsHuman || game.Phase != "night_witch" {
		return fmt.Errorf("当前不是女巫行动回合")
	}
	if req.UseHeal && game.WitchHealUsed {
		return fmt.Errorf("解药已用")
	}
	if req.UsePoison {
		if game.WitchPoisonUsed {
			return fmt.Errorf("毒药已用")
		}
		if req.TargetSeat == nil || !containsInt(s.allowedTargets(game, actor), *req.TargetSeat) {
			return fmt.Errorf("毒药目标不合法")
		}
	}
	if !req.UseHeal && !req.UsePoison {
		return fmt.Errorf("请至少选择一种行动，或使用 pass")
	}
	s.applyWitch(game, actor, req.UseHeal, req.TargetSeat)
	s.runAIUntilPause(game)
	return nil
}

func (s *Service) submitHumanPass(game *gameState) error {
	actor := s.currentActor(game)
	if actor < 0 || !game.Players[actor].IsHuman {
		return fmt.Errorf("当前不是你的行动回合")
	}
	if game.Phase != "night_witch" {
		return fmt.Errorf("当前阶段不能 pass")
	}
	s.applyWitch(game, actor, false, nil)
	s.runAIUntilPause(game)
	return nil
}

func (s *Service) runOneStep(game *gameState) error {
	if game.Status != "running" {
		return nil
	}
	actor := s.currentActor(game)
	if actor < 0 {
		return fmt.Errorf("当前没有可推进的行动")
	}
	s.applyAIAction(game, actor)
	return nil
}

func (s *Service) runAIUntilPause(game *gameState) {
	for game.Status == "running" {
		actor := s.currentActor(game)
		if actor < 0 {
			return
		}
		if game.Players[actor].IsHuman {
			return
		}
		s.applyAIAction(game, actor)
	}
}

func (s *Service) applyAIAction(game *gameState, actor int) {
	if game.Players[actor].Preset.UsesLLM() {
		if handled, err := s.applyLLMAction(game, actor); handled {
			return
		} else if err != nil {
			s.appendEvent(game, "ai_fallback_used", "public", map[string]any{
				"day":        game.Day,
				"phase":      game.Phase,
				"seat":       actor,
				"playerName": game.Players[actor].Name,
				"model":      game.Players[actor].Preset.Model,
				"error":      err.Error(),
			})
		}
	}
	switch game.Phase {
	case "day_main":
		s.applySpeech(game, actor, s.aiMainSpeech(game, actor))
	case "day_reply":
		target := s.pickTarget(game, actor, s.allowedTargets(game, actor), "reply")
		text := s.aiReplySpeech(game, actor, target)
		s.applySpeech(game, actor, text)
	case "day_vote", "night_wolf", "night_seer", "night_guard", "hunter_shot":
		target := s.pickTarget(game, actor, s.allowedTargets(game, actor), game.Phase)
		s.applyTargetAction(game, actor, target)
	case "night_witch":
		useHeal, poisonTarget := s.aiWitchAction(game, actor)
		s.applyWitch(game, actor, useHeal, poisonTarget)
	}
}

func (s *Service) applyLLMAction(game *gameState, actor int) (bool, error) {
	preset := game.Players[actor].Preset
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	decision, err := s.aiClient.Decide(ctx, preset, s.buildLLMTurnInput(game, actor))
	if err != nil {
		return false, err
	}
	switch game.Phase {
	case "day_main", "day_reply":
		speech := strings.TrimSpace(decision.Speech)
		if speech == "" {
			return false, fmt.Errorf("empty speech")
		}
		s.appendEvent(game, "ai_turn_generated", "public", map[string]any{
			"day":        game.Day,
			"phase":      game.Phase,
			"seat":       actor,
			"playerName": game.Players[actor].Name,
			"model":      preset.Model,
			"kind":       "speech",
		})
		s.applySpeech(game, actor, speech)
		return true, nil
	case "day_vote", "night_wolf", "night_seer", "night_guard", "hunter_shot":
		if decision.TargetSeat == nil {
			return false, fmt.Errorf("missing target")
		}
		if !containsInt(s.allowedTargets(game, actor), *decision.TargetSeat) {
			return false, fmt.Errorf("invalid target seat %d", *decision.TargetSeat)
		}
		s.appendEvent(game, "ai_turn_generated", "public", map[string]any{
			"day":        game.Day,
			"phase":      game.Phase,
			"seat":       actor,
			"playerName": game.Players[actor].Name,
			"model":      preset.Model,
			"kind":       "target",
			"targetSeat": *decision.TargetSeat,
		})
		s.applyTargetAction(game, actor, *decision.TargetSeat)
		return true, nil
	case "night_witch":
		if decision.UseHeal && (game.WitchHealUsed || game.PendingNightKill < 0) {
			decision.UseHeal = false
		}
		if decision.PoisonTargetSeat != nil {
			if game.WitchPoisonUsed {
				return false, fmt.Errorf("poison not allowed")
			}
			if !containsInt(s.allowedTargets(game, actor), *decision.PoisonTargetSeat) {
				return false, fmt.Errorf("invalid poison target %d", *decision.PoisonTargetSeat)
			}
		}
		s.appendEvent(game, "ai_turn_generated", "public", map[string]any{
			"day":        game.Day,
			"phase":      game.Phase,
			"seat":       actor,
			"playerName": game.Players[actor].Name,
			"model":      preset.Model,
			"kind":       "witch",
			"useHeal":    decision.UseHeal,
		})
		s.applyWitch(game, actor, decision.UseHeal, decision.PoisonTargetSeat)
		return true, nil
	default:
		return false, fmt.Errorf("unsupported phase %s", game.Phase)
	}
}

func (s *Service) buildLLMTurnInput(game *gameState, actor int) backendai.TurnInput {
	input := backendai.TurnInput{
		Objective:      s.turnObjective(game, actor),
		Template:       game.Template.Name,
		Day:            game.Day,
		Phase:          game.Phase,
		YourSeat:       actor,
		YourName:       game.Players[actor].Name,
		YourRole:       roleDisplay(game.Players[actor].Role),
		Persona:        strings.TrimSpace(game.Players[actor].Preset.Persona),
		AlivePlayers:   s.turnPlayers(game),
		PublicLog:      s.visibleRoundLogForAI(game, actor),
		PrivateNotes:   s.privateNotesForSeat(game, actor),
		ValidTargets:   s.turnTargets(game, actor),
		AllowSpeech:    game.Phase == "day_main" || game.Phase == "day_reply",
		AllowTarget:    game.Phase == "day_vote" || game.Phase == "night_wolf" || game.Phase == "night_seer" || game.Phase == "night_guard" || game.Phase == "hunter_shot",
		AllowWitchMode: game.Phase == "night_witch",
	}
	if game.Phase == "night_witch" && game.PendingNightKill >= 0 {
		input.NightVictim = &backendai.TurnTarget{Seat: game.PendingNightKill, Name: game.Players[game.PendingNightKill].Name}
		input.CanUseHeal = !game.WitchHealUsed
		input.CanUsePoison = !game.WitchPoisonUsed
	}
	if input.Persona == "" {
		input.Persona = strings.TrimSpace(game.Players[actor].Preset.Name)
	}
	return input
}

func (s *Service) visibleRoundLogForAI(game *gameState, actor int) []string {
	startSeq := roundStartSequence(game)
	log := make([]string, 0, len(game.Events))
	for _, event := range game.Events {
		if event.Sequence < startSeq {
			continue
		}
		if !visibleToSeat(event, game, actor) {
			continue
		}
		if item := summarizeVisibleEvent(event); item != "" {
			log = append(log, item)
		}
	}
	return log
}

func roundStartSequence(game *gameState) int {
	if game == nil || len(game.Events) == 0 {
		return 1
	}
	targetPhase := game.Phase
	switch {
	case strings.HasPrefix(game.Phase, "day_"):
		targetPhase = "day_main"
	case strings.HasPrefix(game.Phase, "night_") || game.Phase == "night":
		targetPhase = "night"
	}
	for index := len(game.Events) - 1; index >= 0; index-- {
		event := game.Events[index]
		if event.Type != "phase_started" {
			continue
		}
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			continue
		}
		phase, _ := payload["phase"].(string)
		if strings.TrimSpace(phase) == targetPhase {
			return event.Sequence
		}
	}
	return 1
}

func visibleToSeat(event Event, game *gameState, actor int) bool {
	if event.Visibility == "public" {
		return true
	}
	if actor < 0 || actor >= len(game.Players) {
		return false
	}
	role := game.Players[actor].Role
	switch event.Visibility {
	case "private:wolf":
		return role == RoleWerewolf
	case "private:seer":
		return role == RoleSeer
	case "private:witch":
		return role == RoleWitch
	case "private:guard":
		return role == RoleGuard
	default:
		return false
	}
}

func (s *Service) turnPlayers(game *gameState) []backendai.TurnPlayer {
	players := make([]backendai.TurnPlayer, 0, len(game.Players))
	for _, player := range game.Players {
		revealed := roleDisplayText(player.RevealedRole)
		players = append(players, backendai.TurnPlayer{
			Seat:         player.Seat,
			Name:         player.Name,
			Alive:        player.Alive,
			RevealedRole: revealed,
		})
	}
	return players
}

func (s *Service) turnTargets(game *gameState, actor int) []backendai.TurnTarget {
	seats := s.allowedTargets(game, actor)
	targets := make([]backendai.TurnTarget, 0, len(seats))
	for _, seat := range seats {
		targets = append(targets, backendai.TurnTarget{Seat: seat, Name: game.Players[seat].Name})
	}
	return targets
}


func summarizeVisibleEvent(event Event) string {
	if event.Type == "ai_turn_generated" || event.Type == "ai_fallback_used" {
		return ""
	}
	payload, _ := event.Payload.(map[string]any)
	readText := func(key string) string {
		if payload == nil {
			return ""
		}
		if value, ok := payload[key].(string); ok {
			return strings.TrimSpace(value)
		}
		return ""
	}
	readInt := func(key string) int {
		if payload == nil {
			return 0
		}
		switch value := payload[key].(type) {
		case int:
			return value
		case float64:
			return int(value)
		default:
			return 0
		}
	}
	switch event.Type {
	case "speech_recorded":
		return fmt.Sprintf("第%d天%s %s：%s", readInt("day"), readText("phase"), readText("playerName"), readText("text"))
	case "vote_cast":
		return fmt.Sprintf("第%d天投票 %s -> %s", readInt("day"), readText("playerName"), readText("targetName"))
	case "vote_resolved", "night_resolved", "game_finished":
		if summary := readText("summary"); summary != "" {
			return summary
		}
		return event.Type
	case "player_eliminated":
		return fmt.Sprintf("%s 出局，翻牌 %s", readText("playerName"), readText("role"))
	case "phase_started":
		return fmt.Sprintf("进入阶段：%s", readText("phase"))
	case "game_created":
		return fmt.Sprintf("新对局：%s", readText("template"))
	case "wolf_target_selected":
		return fmt.Sprintf("狼人内部：%s 提议刀 %s", readText("playerName"), readText("targetName"))
	case "wolf_target_locked":
		return fmt.Sprintf("狼人内部：最终刀口 %s", readText("targetName"))
	case "seer_checked":
		return fmt.Sprintf("预言家内部：验 %s = %s", readText("targetName"), readText("result"))
	case "witch_action_recorded":
		parts := []string{}
		if heal, ok := payload["useHeal"].(bool); ok && heal {
			parts = append(parts, fmt.Sprintf("用了救药救 %d 号", readInt("healTarget")))
		} else {
			parts = append(parts, "没用救药")
		}
		if poison, ok := payload["usePoison"].(bool); ok && poison {
			parts = append(parts, fmt.Sprintf("用了毒药毒 %d 号", readInt("poisonTarget")))
		} else {
			parts = append(parts, "没用毒药")
		}
		return "女巫内部：" + strings.Join(parts, "，")
	case "guard_selected":
		return fmt.Sprintf("守卫内部：守了 %s", readText("targetName"))
	default:
		return event.Type
	}
}

func (s *Service) turnObjective(game *gameState, actor int) string {
	switch game.Phase {
	case "day_main":
		return "给出本轮主发言，推动白天站边。"
	case "day_reply":
		return "回应桌面其他人的发言。"
	case "day_vote":
		return "从 validTargets 中投出今天要放逐的人。"
	case "night_wolf":
		return "从 validTargets 中选择今晚刀口。"
	case "night_seer":
		return "从 validTargets 中选择今晚查验对象。"
	case "night_guard":
		return "从 validTargets 中选择今晚守护对象。"
	case "night_witch":
		return "决定今晚是否救人，以及是否毒人。"
	case "hunter_shot":
		return "你已死亡，选择是否开枪以及开枪目标。"
	default:
		return fmt.Sprintf("完成当前阶段 %s 的动作。", game.Phase)
	}
}

func (s *Service) applySpeech(game *gameState, actor int, text string) {
	game.Players[actor].LastSpeech = text
	payload := map[string]any{
		"day":        game.Day,
		"phase":      game.Phase,
		"seat":       actor,
		"playerName": game.Players[actor].Name,
		"text":       text,
	}
	s.appendEvent(game, "speech_recorded", "public", payload)
	s.advanceActor(game)
	if game.Phase == "day_main" && game.PhaseIndex >= len(game.PhaseActors) {
		s.startDayReply(game)
		return
	}
	if game.Phase == "day_reply" && game.PhaseIndex >= len(game.PhaseActors) {
		s.startDayVote(game)
	}
}

func (s *Service) applyTargetAction(game *gameState, actor int, target int) {
	switch game.Phase {
	case "day_vote":
		game.Players[actor].LastVote = target
		s.appendEvent(game, "vote_cast", "public", map[string]any{
			"day":        game.Day,
			"seat":       actor,
			"playerName": game.Players[actor].Name,
			"targetSeat": target,
			"targetName": game.Players[target].Name,
		})
		s.advanceActor(game)
		if game.PhaseIndex >= len(game.PhaseActors) {
			s.resolveVote(game)
		}
	case "night_wolf":
		game.NightWolfVotes[actor] = target
		s.appendEvent(game, "wolf_target_selected", "private:wolf", map[string]any{
			"day":        game.Day,
			"seat":       actor,
			"playerName": game.Players[actor].Name,
			"targetSeat": target,
			"targetName": game.Players[target].Name,
			"reason":     s.shortReason(game, actor, target, "night_wolf"),
		})
		s.advanceActor(game)
		if game.PhaseIndex >= len(game.PhaseActors) {
			game.PendingNightKill = s.resolveTargetSelections(game.NightWolfVotes)
			s.appendEvent(game, "wolf_target_locked", "private:wolf", map[string]any{
				"day":        game.Day,
				"targetSeat": game.PendingNightKill,
				"targetName": game.Players[game.PendingNightKill].Name,
			})
			s.startNightSeer(game)
		}
	case "night_seer":
		game.SeerChecked[target] = struct{}{}
		game.SeerFindings[target] = game.Players[target].Role
		s.appendEvent(game, "seer_checked", "private:seer", map[string]any{
			"day":        game.Day,
			"seat":       actor,
			"targetSeat": target,
			"targetName": game.Players[target].Name,
			"result":     roleDisplay(game.Players[target].Role),
		})
		s.advanceActor(game)
		s.startNightWitch(game)
	case "night_guard":
		game.NightGuardTarget = target
		s.appendEvent(game, "guard_selected", "private:guard", map[string]any{
			"day":        game.Day,
			"seat":       actor,
			"targetSeat": target,
			"targetName": game.Players[target].Name,
		})
		s.advanceActor(game)
		s.startNightWolf(game)
	case "hunter_shot":
		s.appendEvent(game, "hunter_shot", "public", map[string]any{
			"day":        game.Day,
			"seat":       actor,
			"playerName": game.Players[actor].Name,
			"targetSeat": target,
			"targetName": game.Players[target].Name,
		})
		s.killPlayer(game, target, "hunter_shot")
		game.HunterShotUsed = true
		s.resumeAfterDeaths(game)
	}
}

func (s *Service) applyWitch(game *gameState, actor int, useHeal bool, poisonTarget *int) {
	if useHeal && !game.WitchHealUsed && game.PendingNightKill >= 0 {
		game.WitchHealUsed = true
		game.NightHealTarget = game.PendingNightKill
	}
	if poisonTarget != nil && !game.WitchPoisonUsed {
		game.WitchPoisonUsed = true
		game.NightPoisonTarget = *poisonTarget
	}
	s.appendEvent(game, "witch_action_recorded", "private:witch", map[string]any{
		"day":         game.Day,
		"seat":        actor,
		"useHeal":     useHeal,
		"healTarget":  game.NightHealTarget,
		"usePoison":   poisonTarget != nil,
		"poisonTarget": game.NightPoisonTarget,
	})
	s.advanceActor(game)
	s.resolveNight(game)
}

func (s *Service) resolveVote(game *gameState) {
	votes := map[int]int{}
	for _, actor := range game.PhaseActors {
		target := game.Players[actor].LastVote
		if target >= 0 {
			votes[target]++
		}
	}
	selected, tied := resolveVotes(votes)
	if selected < 0 || tied {
		s.appendEvent(game, "vote_resolved", "public", map[string]any{
			"day":     game.Day,
			"result":  "no_elimination",
			"summary": "平票，本轮不出人。",
		})
		game.PendingResume = "vote"
		if shouldPauseAfterDayVote(game) {
			game.Control.Running = false
			game.Control.Paused = false
			game.Control.CanStep = false
			game.Message = fmt.Sprintf("第 %d 天投票已结算，点击继续下一天", game.Day)
			return
		}
		s.resumePendingState(game)
		return
	}
	s.appendEvent(game, "vote_resolved", "public", map[string]any{
		"day":        game.Day,
		"result":     "eliminated",
		"targetSeat": selected,
		"targetName": game.Players[selected].Name,
	})
	s.killPlayer(game, selected, "vote")
	game.PendingResume = "vote"
	s.resumeAfterDeaths(game)
}

func (s *Service) resolveNight(game *gameState) {
	deaths := map[int]struct{}{}
	if game.PendingNightKill >= 0 && game.PendingNightKill != game.NightGuardTarget && game.PendingNightKill != game.NightHealTarget {
		deaths[game.PendingNightKill] = struct{}{}
	}
	if game.NightPoisonTarget >= 0 {
		deaths[game.NightPoisonTarget] = struct{}{}
	}
	deadNames := make([]string, 0, len(deaths))
	deadSeats := make([]int, 0, len(deaths))
	for seat := range deaths {
		deadSeats = append(deadSeats, seat)
	}
	sort.Ints(deadSeats)
	for _, seat := range deadSeats {
		deadNames = append(deadNames, game.Players[seat].Name)
	}
	if len(deadSeats) == 0 {
		s.appendEvent(game, "night_resolved", "public", map[string]any{
			"day":     game.Day,
			"summary": "昨夜是平安夜。",
		})
	} else {
		s.appendEvent(game, "night_resolved", "public", map[string]any{
			"day":       game.Day,
			"deadSeats": deadSeats,
			"deadNames": deadNames,
			"summary":   fmt.Sprintf("昨夜死亡：%s", strings.Join(deadNames, "、")),
		})
	}
	for _, seat := range deadSeats {
		s.killPlayer(game, seat, "night")
	}
	game.PendingResume = "night"
	game.GuardLastTarget = game.NightGuardTarget
	s.resumeAfterDeaths(game)
}

func (s *Service) resumeAfterDeaths(game *gameState) {
	if winner := s.winnerSide(game); winner != "" {
		s.finishGame(game, winner)
		return
	}
	if hunter := s.pendingHunterSeat(game); hunter >= 0 {
		s.startHunterShot(game, hunter)
		return
	}
	if shouldPauseAfterDayVote(game) {
		game.Control.Running = false
		game.Control.Paused = false
		game.Control.CanStep = false
		game.Message = fmt.Sprintf("第 %d 天投票已结算，点击继续下一天", game.Day)
		return
	}
	s.resumePendingState(game)
}

func (s *Service) resumePendingState(game *gameState) {
	switch game.PendingResume {
	case "night":
		s.startDayMain(game)
	case "vote":
		game.Day++
		s.startNight(game)
	}
	game.PendingResume = ""
}

func shouldPauseAfterDayVote(game *gameState) bool {
	if game == nil {
		return false
	}
	return game.Mode == "spectator" && game.Control.SemiAutoMode && !game.Control.ManualMode && game.PendingResume == "vote"
}

func (s *Service) startNight(game *gameState) {
	game.PendingNightKill = -1
	game.NightHealTarget = -1
	game.NightPoisonTarget = -1
	game.NightGuardTarget = -1
	game.NightWolfVotes = map[int]int{}
	game.Phase = "night"
	game.Message = fmt.Sprintf("第 %d 夜开始", game.Day)
	s.appendEvent(game, "phase_started", "public", map[string]any{"phase": "night", "day": game.Day})
	s.startNightGuard(game)
}

func (s *Service) startNightGuard(game *gameState) {
	actors := s.aliveRoleSeats(game, RoleGuard)
	if len(actors) == 0 {
		s.startNightWolf(game)
		return
	}
	game.Phase = "night_guard"
	game.Message = "夜晚：守卫行动"
	game.PhaseActors = actors
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "private:guard", map[string]any{"phase": "night_guard", "day": game.Day})
	if len(s.allowedTargets(game, actors[0])) == 0 {
		s.startNightWolf(game)
	}
}

func (s *Service) startNightWolf(game *gameState) {
	actors := s.aliveRoleSeats(game, RoleWerewolf)
	if len(actors) == 0 {
		s.startNightSeer(game)
		return
	}
	game.Phase = "night_wolf"
	game.Message = "夜晚：狼人刀人"
	game.PhaseActors = actors
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "private:wolf", map[string]any{"phase": "night_wolf", "day": game.Day})
}

func (s *Service) startNightSeer(game *gameState) {
	actors := s.aliveRoleSeats(game, RoleSeer)
	if len(actors) == 0 {
		s.startNightWitch(game)
		return
	}
	game.Phase = "night_seer"
	game.Message = "夜晚：预言家查验"
	game.PhaseActors = actors
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "private:seer", map[string]any{"phase": "night_seer", "day": game.Day})
}

func (s *Service) startNightWitch(game *gameState) {
	actors := s.aliveRoleSeats(game, RoleWitch)
	if len(actors) == 0 {
		s.resolveNight(game)
		return
	}
	game.Phase = "night_witch"
	game.Message = "夜晚：女巫行动"
	game.PhaseActors = actors
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "private:witch", map[string]any{"phase": "night_witch", "day": game.Day, "wolfTarget": game.PendingNightKill})
}

func (s *Service) startDayMain(game *gameState) {
	actors := s.aliveSeats(game)
	game.Phase = "day_main"
	game.Message = fmt.Sprintf("第 %d 天：主发言", game.Day)
	game.PhaseActors = actors
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "public", map[string]any{"phase": "day_main", "day": game.Day})
}

func (s *Service) startDayReply(game *gameState) {
	actors := s.aliveSeats(game)
	game.Phase = "day_reply"
	game.Message = fmt.Sprintf("第 %d 天：回应轮", game.Day)
	game.PhaseActors = actors
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "public", map[string]any{"phase": "day_reply", "day": game.Day})
}

func (s *Service) startDayVote(game *gameState) {
	actors := s.aliveSeats(game)
	game.Phase = "day_vote"
	game.Message = fmt.Sprintf("第 %d 天：投票", game.Day)
	game.PhaseActors = actors
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "public", map[string]any{"phase": "day_vote", "day": game.Day})
}

func (s *Service) startHunterShot(game *gameState, seat int) {
	game.Phase = "hunter_shot"
	game.Message = fmt.Sprintf("%s 可发动猎人技能", game.Players[seat].Name)
	game.PhaseActors = []int{seat}
	game.PhaseIndex = 0
	s.appendEvent(game, "phase_started", "public", map[string]any{"phase": "hunter_shot", "day": game.Day, "seat": seat, "playerName": game.Players[seat].Name})
}

func (s *Service) finishGame(game *gameState, winner string) {
	game.Status = "finished"
	game.WinnerSide = winner
	game.FinishedAt = time.Now().UTC()
	game.Message = fmt.Sprintf("对局结束：%s胜利", winner)
	s.appendEvent(game, "game_finished", "public", map[string]any{"winnerSide": winner, "summary": fmt.Sprintf("对局结束：%s胜利", winner)})
}

func (s *Service) advanceActor(game *gameState) {
	game.PhaseIndex++
}

func (s *Service) currentActor(game *gameState) int {
	if game.Status != "running" || game.PhaseIndex >= len(game.PhaseActors) {
		return -1
	}
	return game.PhaseActors[game.PhaseIndex]
}

func (s *Service) allowedTargets(game *gameState, actor int) []int {
	switch game.Phase {
	case "day_reply", "day_vote", "hunter_shot":
		return s.aliveOtherSeats(game, actor)
	case "night_wolf":
		out := make([]int, 0)
		for _, seat := range s.aliveOtherSeats(game, actor) {
			if game.Players[seat].Role != RoleWerewolf {
				out = append(out, seat)
			}
		}
		return out
	case "night_seer":
		return s.aliveOtherSeats(game, actor)
	case "night_guard":
		out := make([]int, 0)
		for _, seat := range s.aliveSeats(game) {
			if seat == game.GuardLastTarget && len(s.aliveSeats(game)) > 1 {
				continue
			}
			out = append(out, seat)
		}
		return out
	case "night_witch":
		return s.aliveOtherSeats(game, actor)
	default:
		return nil
	}
}

func (s *Service) aliveSeats(game *gameState) []int {
	out := make([]int, 0, len(game.Players))
	for _, player := range game.Players {
		if player.Alive {
			out = append(out, player.Seat)
		}
	}
	return out
}

func (s *Service) aliveOtherSeats(game *gameState, actor int) []int {
	out := make([]int, 0, len(game.Players))
	for _, player := range game.Players {
		if player.Alive && player.Seat != actor {
			out = append(out, player.Seat)
		}
	}
	return out
}

func (s *Service) aliveRoleSeats(game *gameState, role Role) []int {
	out := []int{}
	for _, player := range game.Players {
		if player.Alive && player.Role == role {
			out = append(out, player.Seat)
		}
	}
	return out
}

func (s *Service) killPlayer(game *gameState, seat int, reason string) {
	if seat < 0 || seat >= len(game.Players) || !game.Players[seat].Alive {
		return
	}
	game.Players[seat].Alive = false
	game.Players[seat].RevealedRole = string(game.Players[seat].Role)
	s.appendEvent(game, "player_eliminated", "public", map[string]any{
		"day":        game.Day,
		"seat":       seat,
		"playerName": game.Players[seat].Name,
		"reason":     reason,
		"role":       roleDisplay(game.Players[seat].Role),
	})
}

func (s *Service) winnerSide(game *gameState) string {
	wolves := 0
	villagers := 0
	for _, player := range game.Players {
		if !player.Alive {
			continue
		}
		if player.Role == RoleWerewolf {
			wolves++
		} else {
			villagers++
		}
	}
	if wolves == 0 {
		return "好人阵营"
	}
	if wolves >= villagers {
		return "狼人阵营"
	}
	return ""
}

func (s *Service) pendingHunterSeat(game *gameState) int {
	if game.HunterShotUsed {
		return -1
	}
	for _, player := range game.Players {
		if player.Role == RoleHunter && !player.Alive {
			return player.Seat
		}
	}
	return -1
}

func (s *Service) majorityTarget(votes map[int]int) int {
	bestSeat := -1
	bestVotes := -1
	for seat, count := range votes {
		if count > bestVotes || (count == bestVotes && seat < bestSeat) {
			bestSeat = seat
			bestVotes = count
		}
	}
	return bestSeat
}

func (s *Service) resolveTargetSelections(selections map[int]int) int {
	counts := map[int]int{}
	for _, target := range selections {
		counts[target]++
	}
	return s.majorityTarget(counts)
}

func (s *Service) appendEvent(game *gameState, typ string, visibility string, payload any) {
	game.Sequence++
	event := Event{Sequence: game.Sequence, Type: typ, Visibility: visibility, Timestamp: time.Now().UTC(), Payload: payload}
	game.Events = append(game.Events, event)
	game.UpdatedAt = event.Timestamp
}

func (s *Service) snapshotLocked(game *gameState) Snapshot {
	players := make([]Player, len(game.Players))
	includePrivate := game.Mode == "spectator"
	for idx, player := range game.Players {
		players[idx] = Player{
			Seat:         player.Seat,
			Name:         player.Name,
			Alive:        player.Alive,
			IsHuman:      player.IsHuman,
			PresetID:     player.Preset.ID,
			RevealedRole: roleDisplayText(player.RevealedRole),
		}
		if includePrivate {
			players[idx].Role = roleDisplay(player.Role)
		}
	}
	var heroSeat *int
	var heroState *HeroState
	if game.Mode == "human" && game.HeroSeat >= 0 {
		seat := game.HeroSeat
		heroSeat = &seat
		heroState = &HeroState{Role: roleDisplay(game.Players[seat].Role), Notes: s.heroNotes(game)}
	}
	return Snapshot{
		ID:            game.ID,
		Status:        statusForGame(game),
		Mode:          game.Mode,
		Template:      game.Template,
		Day:           game.Day,
		Phase:         game.Phase,
		Message:       game.Message,
		Players:       players,
		HeroSeat:      heroSeat,
		HeroState:     heroState,
		Control:       game.Control,
		PendingAction: s.pendingAction(game),
		Events:        visibleEvents(game, includePrivate),
		WinnerSide:    game.WinnerSide,
		CreatedAt:     game.CreatedAt,
		UpdatedAt:     game.UpdatedAt,
		FinishedAt:    game.FinishedAt,
	}
}

func (s *Service) pendingAction(game *gameState) *PendingAction {
	actor := s.currentActor(game)
	if actor < 0 || game.Status != "running" {
		return nil
	}
	if game.Mode == "spectator" {
		return &PendingAction{Kind: "continue", ActorSeat: actor, Prompt: fmt.Sprintf("下一步将由 %s 处理 %s。", game.Players[actor].Name, phaseDisplay(game.Phase))}
	}
	if !game.Players[actor].IsHuman {
		return &PendingAction{Kind: "continue", ActorSeat: actor, Prompt: fmt.Sprintf("%s 正在思考。", game.Players[actor].Name)}
	}
	switch game.Phase {
	case "day_main":
		return &PendingAction{Kind: "speech", ActorSeat: actor, Prompt: "轮到你做主发言。", AllowText: true, Placeholder: "输入你的主发言"}
	case "day_reply":
		return &PendingAction{Kind: "speech", ActorSeat: actor, Prompt: "轮到你回应桌面。", AllowText: true, Placeholder: "输入你的回应"}
	case "day_vote":
		return &PendingAction{Kind: "select", ActorSeat: actor, Prompt: "请选择今天要放逐的目标。", Options: seatOptions(game, s.allowedTargets(game, actor))}
	case "night_wolf":
		return &PendingAction{Kind: "select", ActorSeat: actor, Prompt: "请选择今晚刀口。", Options: seatOptions(game, s.allowedTargets(game, actor))}
	case "night_seer":
		return &PendingAction{Kind: "select", ActorSeat: actor, Prompt: "请选择今晚要查验的人。", Options: seatOptions(game, s.allowedTargets(game, actor))}
	case "night_guard":
		return &PendingAction{Kind: "select", ActorSeat: actor, Prompt: "请选择今晚守护的人。", Options: seatOptions(game, s.allowedTargets(game, actor))}
	case "night_witch":
		prompt := "请选择是否使用药剂。"
		if game.PendingNightKill >= 0 {
			prompt = fmt.Sprintf("今晚刀口是 %s。可选择救或毒。", game.Players[game.PendingNightKill].Name)
		}
		return &PendingAction{Kind: "witch", ActorSeat: actor, Prompt: prompt, Options: seatOptions(game, s.allowedTargets(game, actor)), AllowPass: true, AllowHeal: !game.WitchHealUsed && game.PendingNightKill >= 0}
	case "hunter_shot":
		return &PendingAction{Kind: "select", ActorSeat: actor, Prompt: "你已出局，可选择开枪目标。", Options: seatOptions(game, s.allowedTargets(game, actor))}
	default:
		return nil
	}
}

func (s *Service) recordSummaryLocked(game *gameState) RecordSummary {
	return RecordSummary{ID: game.ID, Status: statusForGame(game), Mode: game.Mode, TemplateName: game.Template.Name, Day: game.Day, WinnerSide: game.WinnerSide, CreatedAt: game.CreatedAt, UpdatedAt: game.UpdatedAt, FinishedAt: game.FinishedAt}
}

func statusForGame(game *gameState) string {
	if game == nil {
		return "stopped"
	}
	if game.Status == "finished" || game.WinnerSide != "" {
		return "finished"
	}
	if game.Status == "stopped" || game.Control.Stopped {
		return "stopped"
	}
	actor := -1
	if game.Status == "running" && game.PhaseIndex < len(game.PhaseActors) {
		actor = game.PhaseActors[game.PhaseIndex]
	}
	if actor >= 0 && actor < len(game.Players) && game.Players[actor].IsHuman {
		return "awaiting_human"
	}
	if game.Mode == "spectator" && !game.Control.Running {
		return "paused"
	}
	return game.Status
}

func (s *Service) heroNotes(game *gameState) []string {
	return s.privateNotesForSeat(game, game.HeroSeat)
}

func (s *Service) privateNotesForSeat(game *gameState, seat int) []string {
	if seat < 0 || seat >= len(game.Players) {
		return nil
	}
	hero := game.Players[seat]
	notes := []string{}
	switch hero.Role {
	case RoleWerewolf:
		mates := []string{}
		for _, player := range game.Players {
			if player.Seat != hero.Seat && player.Role == RoleWerewolf {
				mates = append(mates, fmt.Sprintf("%s(%d号)", player.Name, player.Seat))
			}
		}
		if len(mates) > 0 {
			notes = append(notes, "狼队友："+strings.Join(mates, "、"))
		}
	case RoleSeer:
		if len(game.SeerFindings) == 0 {
			notes = append(notes, "你还没有查验结果。")
		}
		seats := make([]int, 0, len(game.SeerFindings))
		for seat := range game.SeerFindings {
			seats = append(seats, seat)
		}
		sort.Ints(seats)
		for _, seat := range seats {
			notes = append(notes, fmt.Sprintf("你验过 %s：%s", game.Players[seat].Name, roleDisplay(game.SeerFindings[seat])))
		}
	case RoleWitch:
		if game.WitchHealUsed {
			notes = append(notes, "解药：已使用")
		} else {
			notes = append(notes, "解药：未使用")
		}
		if game.WitchPoisonUsed {
			notes = append(notes, "毒药：已使用")
		} else {
			notes = append(notes, "毒药：未使用")
		}
		if game.Phase == "night_witch" && game.PendingNightKill >= 0 {
			notes = append(notes, fmt.Sprintf("今晚刀口：%s", game.Players[game.PendingNightKill].Name))
		}
	case RoleGuard:
		if game.GuardLastTarget >= 0 {
			notes = append(notes, fmt.Sprintf("上晚守护：%s", game.Players[game.GuardLastTarget].Name))
		}
	}
	return notes
}

func (s *Service) pickTarget(game *gameState, actor int, options []int, phase string) int {
	if len(options) == 0 {
		return -1
	}
	best := options[0]
	bestScore := -1
	for _, target := range options {
		score := game.RNG.Intn(4)
		role := game.Players[actor].Role
		targetRole := game.Players[target].Role
		switch phase {
		case "reply":
			score += 1
		case "day_vote":
			if role == RoleWerewolf && targetRole != RoleWerewolf {
				score += 6
			}
			if role != RoleWerewolf && targetRole == RoleWerewolf {
				score += 4
			}
			if role == RoleSeer {
				if found, ok := game.SeerFindings[target]; ok && found == RoleWerewolf {
					score += 10
				}
			}
		case "night_wolf":
			if targetRole == RoleSeer {
				score += 6
			} else if targetRole == RoleWitch {
				score += 5
			} else if targetRole == RoleHunter || targetRole == RoleGuard {
				score += 4
			}
		case "night_seer":
			if _, seen := game.SeerChecked[target]; !seen {
				score += 5
			}
		case "night_guard":
			if targetRole == RoleSeer || targetRole == RoleWitch {
				score += 4
			}
		case "hunter_shot":
			if targetRole == RoleWerewolf {
				score += 8
			}
		}
		if score > bestScore || (score == bestScore && target < best) {
			best = target
			bestScore = score
		}
	}
	return best
}

func (s *Service) aiMainSpeech(game *gameState, actor int) string {
	target := s.pickTarget(game, actor, s.aliveOtherSeats(game, actor), "day_vote")
	name := game.Players[target].Name
	styleLead := styleLead(game.Players[actor].Preset.Style)
	switch game.Players[actor].Role {
	case RoleSeer:
		for seat, role := range game.SeerFindings {
			if role == RoleWerewolf && game.Players[seat].Alive {
				return fmt.Sprintf("%s，昨晚我验了 %s，是狼人，今天先出这边。", styleLead, game.Players[seat].Name)
			}
		}
		for seat, role := range game.SeerFindings {
			if role != RoleWerewolf {
				return fmt.Sprintf("%s，昨晚我验了 %s，是好人，我更想听 %s 的解释。", styleLead, game.Players[seat].Name, name)
			}
		}
	case RoleWerewolf:
		return fmt.Sprintf("%s，我先听 %s 这轮怎么盘，当前不想太早站死边。", styleLead, name)
	case RoleWitch:
		return fmt.Sprintf("%s，桌面信息还不够满，我先看 %s 的发言落点。", styleLead, name)
	case RoleHunter:
		return fmt.Sprintf("%s，今天我先盯 %s，票型别乱飞。", styleLead, name)
	case RoleGuard:
		return fmt.Sprintf("%s，先把 %s 的逻辑过一遍，再看后置位怎么接。", styleLead, name)
	}
	return fmt.Sprintf("%s，我现在更怀疑 %s，后面想看票型怎么落。", styleLead, name)
}

func (s *Service) aiReplySpeech(game *gameState, actor int, target int) string {
	styleLead := styleLead(game.Players[actor].Preset.Style)
	return fmt.Sprintf("%s，我回应一下 %s：你这轮的站边还是偏浮，我暂时不会把你放干净。", styleLead, game.Players[target].Name)
}

func (s *Service) aiWitchAction(game *gameState, actor int) (bool, *int) {
	useHeal := false
	var poison *int
	if !game.WitchHealUsed && game.PendingNightKill >= 0 {
		targetRole := game.Players[game.PendingNightKill].Role
		if targetRole == RoleSeer || targetRole == RoleHunter || targetRole == RoleWitch || game.PendingNightKill == actor {
			useHeal = true
		}
	}
	if !game.WitchPoisonUsed && game.RNG.Intn(100) < 25 {
		candidates := s.allowedTargets(game, actor)
		if len(candidates) > 0 {
			target := s.pickTarget(game, actor, candidates, "day_vote")
			poison = &target
		}
	}
	return useHeal, poison
}

func (s *Service) shortReason(game *gameState, actor int, target int, phase string) string {
	switch phase {
	case "night_wolf":
		return fmt.Sprintf("优先压 %s 的身份位。", roleDisplay(game.Players[target].Role))
	default:
		return fmt.Sprintf("当前更偏向 %s。", game.Players[target].Name)
	}
}

func visibleEvents(game *gameState, includePrivate bool) []Event {
	out := make([]Event, 0, len(game.Events))
	for _, event := range game.Events {
		if includePrivate || event.Visibility == "public" {
			out = append(out, event)
		}
	}
	return out
}

func visibleNewEvents(game *gameState, oldSeq int, includePrivate bool) []Event {
	out := []Event{}
	for _, event := range game.Events {
		if event.Sequence <= oldSeq {
			continue
		}
		if includePrivate || event.Visibility == "public" {
			out = append(out, event)
		}
	}
	return out
}

func playerNames(players []playerState) []string {
	out := make([]string, len(players))
	for i, player := range players {
		out[i] = player.Name
	}
	return out
}

func seatOptions(game *gameState, seats []int) []ActionOption {
	options := make([]ActionOption, 0, len(seats))
	for _, seat := range seats {
		options = append(options, ActionOption{Value: fmt.Sprintf("%d", seat), Label: fmt.Sprintf("%s（%d号）", game.Players[seat].Name, seat)})
	}
	return options
}

func resolveVotes(votes map[int]int) (int, bool) {
	selected := -1
	best := -1
	tied := false
	for seat, count := range votes {
		if count > best {
			selected = seat
			best = count
			tied = false
			continue
		}
		if count == best {
			tied = true
			if seat < selected {
				selected = seat
			}
		}
	}
	return selected, tied
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func roleDisplay(role Role) string {
	switch role {
	case RoleWerewolf:
		return "狼人"
	case RoleSeer:
		return "预言家"
	case RoleWitch:
		return "女巫"
	case RoleHunter:
		return "猎人"
	case RoleGuard:
		return "守卫"
	default:
		return "平民"
	}
}

func roleDisplayText(role string) string {
	switch Role(role) {
	case RoleWerewolf:
		return "狼人"
	case RoleSeer:
		return "预言家"
	case RoleWitch:
		return "女巫"
	case RoleHunter:
		return "猎人"
	case RoleGuard:
		return "守卫"
	case RoleVillager:
		return "平民"
	default:
		return ""
	}
}

func styleLead(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "pressure":
		return "我先给一个压力位"
	case "chaos":
		return "这轮我先把桌面搅开"
	default:
		return "我先收一下桌面信息"
	}
}

func phaseDisplay(phase string) string {
	switch phase {
	case "day_main":
		return "主发言"
	case "day_reply":
		return "回应"
	case "day_vote":
		return "投票"
	case "night_wolf":
		return "狼人夜刀"
	case "night_seer":
		return "预言家查验"
	case "night_witch":
		return "女巫行动"
	case "night_guard":
		return "守卫行动"
	case "hunter_shot":
		return "猎人开枪"
	default:
		return phase
	}
}

func newID() (string, error) {
	buf := make([]byte, 8)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
