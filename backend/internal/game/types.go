package game

import "time"

type Role string

const (
	RoleWerewolf Role = "werewolf"
	RoleSeer     Role = "seer"
	RoleWitch    Role = "witch"
	RoleHunter   Role = "hunter"
	RoleGuard    Role = "guard"
	RoleVillager Role = "villager"
)

type Template struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Seats int    `json:"seats"`
	Roles []Role `json:"roles"`
}

type Player struct {
	Seat         int    `json:"seat"`
	Name         string `json:"name"`
	Alive        bool   `json:"alive"`
	IsHuman      bool   `json:"isHuman"`
	PresetID     string `json:"presetId,omitempty"`
	Role         string `json:"role,omitempty"`
	RevealedRole string `json:"revealedRole,omitempty"`
}

type ActionOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type PendingAction struct {
	Kind        string         `json:"kind"`
	ActorSeat   int            `json:"actorSeat"`
	Prompt      string         `json:"prompt"`
	Options     []ActionOption `json:"options,omitempty"`
	AllowText   bool           `json:"allowText,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	AllowPass   bool           `json:"allowPass,omitempty"`
	AllowHeal   bool           `json:"allowHeal,omitempty"`
}

type Event struct {
	Sequence   int       `json:"sequence"`
	Type       string    `json:"type"`
	Visibility string    `json:"visibility"`
	Timestamp  time.Time `json:"timestamp"`
	Payload    any       `json:"payload"`
}

type HeroState struct {
	Role  string   `json:"role,omitempty"`
	Notes []string `json:"notes,omitempty"`
}

type ControlState struct {
	SpectatorMode bool `json:"spectatorMode"`
	SemiAutoMode  bool `json:"semiAutoMode"`
	Paused        bool `json:"paused"`
	ManualMode    bool `json:"manualMode"`
	CanStep       bool `json:"canStep"`
	Stopped       bool `json:"stopped"`
	Running       bool `json:"running"`
}

type Snapshot struct {
	ID            string         `json:"id"`
	Status        string         `json:"status"`
	Mode          string         `json:"mode"`
	Template      Template       `json:"template"`
	Day           int            `json:"day"`
	Phase         string         `json:"phase"`
	Message       string         `json:"message"`
	Players       []Player       `json:"players"`
	HeroSeat      *int           `json:"heroSeat,omitempty"`
	HeroState     *HeroState     `json:"heroState,omitempty"`
	Control       ControlState   `json:"control"`
	PendingAction *PendingAction `json:"pendingAction,omitempty"`
	Events        []Event        `json:"events"`
	WinnerSide    string         `json:"winnerSide,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	FinishedAt    time.Time      `json:"finishedAt,omitempty"`
}

type CreateRequest struct {
	TemplateID    string   `json:"templateId"`
	SpectatorMode bool     `json:"spectatorMode"`
	SemiAutoMode  bool     `json:"semiAutoMode,omitempty"`
	ManualMode    bool     `json:"manualMode,omitempty"`
	HumanName     string   `json:"humanName,omitempty"`
	AIPresetIDs   []string `json:"aiPresetIds"`
	Seed          int64    `json:"seed,omitempty"`
}

type ControlRequest struct {
	Action string `json:"action"`
}

type ActionRequest struct {
	Action    string `json:"action"`
	Text      string `json:"text,omitempty"`
	TargetSeat *int  `json:"targetSeat,omitempty"`
	UseHeal   bool   `json:"useHeal,omitempty"`
	UsePoison bool   `json:"usePoison,omitempty"`
}

type RecordSummary struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	Mode         string    `json:"mode"`
	TemplateName string    `json:"templateName"`
	Day          int       `json:"day"`
	WinnerSide   string    `json:"winnerSide,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	FinishedAt   time.Time `json:"finishedAt,omitempty"`
}

type ReplayPlayer struct {
	Seat     int    `json:"seat"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Alive    bool   `json:"alive"`
	IsHuman  bool   `json:"isHuman"`
	PresetID string `json:"presetId,omitempty"`
}

type ReplayDetail struct {
	Summary  RecordSummary `json:"summary"`
	Players  []ReplayPlayer `json:"players"`
	Events   []Event       `json:"events"`
	Template Template      `json:"template"`
	CreatedAt time.Time    `json:"createdAt"`
}
