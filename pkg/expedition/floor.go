package expedition

import (
	"errors"
	"fmt"
	"sync"
)

type PlayerID string
type EncounterID string

type Payment uint8

const (
	Personal Payment = iota + 1
	Shared
)

type EncounterKind uint8

const (
	Ordinary EncounterKind = iota + 1
	NormalBoss
)

type BossRoute string

const (
	NoBossRoute       BossRoute = ""
	NormalBossRoute   BossRoute = "normal"
	EnhancedBossRoute BossRoute = "enhanced"
)

type Phase string

const (
	Exploring         Phase = "exploring"
	ExhaustedSettling Phase = "exhausted_settling"
	EnhancedBossReady Phase = "enhanced_boss_ready"
	BossCommitted     Phase = "boss_committed"
)

var (
	ErrInvalid           = errors.New("invalid request")
	ErrUnknownPlayer     = errors.New("unknown player")
	ErrInsufficient      = errors.New("insufficient action points")
	ErrPlayerBusy        = errors.New("player already participating in an encounter")
	ErrDuplicate         = errors.New("encounter or decision already used")
	ErrApprovalRequired  = errors.New("shared spend requires team authorization")
	ErrApprovalDenied    = errors.New("shared spend was not authorized")
	ErrBossCommitted     = errors.New("boss route already committed")
	ErrEnhancedNotReady  = errors.New("enhanced boss requires exhaustion and settled encounters")
	ErrEncounterNotFound = errors.New("active encounter not found")
)

// SharedSpend identifies the exact proposal the server must have approved.
// Participants are intentionally independent of approval to open the node.
type SharedSpend struct {
	FloorID     string
	DecisionID  string
	EncounterID EncounterID
	Kind        EncounterKind
	Cost        int64
}

// SharedAuthorizer verifies an existing team decision without consuming it.
// Authorize runs inside the floor transaction: implementations must be quick,
// concurrency-safe across floors, and must not call back into this Floor.
// The floor records a decision as used only after the whole entry succeeds.
type SharedAuthorizer interface {
	Authorize(SharedSpend) error
}

type Config struct {
	ID              string
	Players         []PlayerID
	PersonalInitial int64
	SharedInitial   int64
	Authorizer      SharedAuthorizer
}

// Entry represents an admitted group. Cost is per entrant for Personal and
// once per opening for Shared. Both paths require a positive configured cost.
// This is an internal server API, not a request to trust directly from clients.
type Entry struct {
	ID         EncounterID
	Kind       EncounterKind
	Players    []PlayerID
	Payment    Payment
	Cost       int64
	DecisionID string
}

type Encounter struct {
	Kind    EncounterKind
	Players []PlayerID
}

// Snapshot is a detached copy. Mutating it cannot change the floor.
type Snapshot struct {
	FloorID       string
	Personal      map[PlayerID]int64
	Shared        int64
	Active        map[EncounterID]Encounter
	Phase         Phase
	BossRoute     BossRoute
	BossEncounter EncounterID
}

// Floor serializes admission, spending, and settlement into atomic operations.
// Do not copy a Floor; construct it with NewFloor and use its pointer.
type Floor struct {
	mu            sync.Mutex
	id            string
	personal      map[PlayerID]int64
	shared        int64
	authorizer    SharedAuthorizer
	active        map[EncounterID]Encounter
	usedIDs       map[EncounterID]struct{}
	usedDecisions map[string]struct{}
	busy          map[PlayerID]EncounterID
	bossRoute     BossRoute
	bossEncounter EncounterID
}

func NewFloor(config Config) (*Floor, error) {
	if config.ID == "" || len(config.Players) == 0 || config.PersonalInitial < 0 || config.SharedInitial < 0 {
		return nil, fmt.Errorf("%w: floor ID, players, and nonnegative initial values are required", ErrInvalid)
	}
	f := &Floor{
		id: config.ID, shared: config.SharedInitial, authorizer: config.Authorizer,
		personal: make(map[PlayerID]int64, len(config.Players)),
		active:   make(map[EncounterID]Encounter), usedIDs: make(map[EncounterID]struct{}),
		usedDecisions: make(map[string]struct{}), busy: make(map[PlayerID]EncounterID),
	}
	for _, player := range config.Players {
		if player == "" {
			return nil, fmt.Errorf("%w: empty player ID", ErrInvalid)
		}
		if _, exists := f.personal[player]; exists {
			return nil, fmt.Errorf("%w: duplicate player %q", ErrInvalid, player)
		}
		f.personal[player] = config.PersonalInitial
	}
	return f, nil
}

// Enter validates the entire admission before spending anything. Shared
// spending may consume the last point for any node, including a normal boss.
// Successful normal-boss admission commits that route before exhaustion is
// observed; later settlement can never turn it into an enhanced boss.
func (f *Floor) Enter(entry Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.bossRoute != NoBossRoute {
		return ErrBossCommitted
	}
	if entry.ID == "" || len(entry.Players) == 0 || entry.Cost <= 0 {
		return fmt.Errorf("%w: encounter ID, participants, and positive cost are required", ErrInvalid)
	}
	if entry.Kind != Ordinary && entry.Kind != NormalBoss {
		return fmt.Errorf("%w: encounter kind", ErrInvalid)
	}
	if entry.Payment != Personal && entry.Payment != Shared {
		return fmt.Errorf("%w: payment kind", ErrInvalid)
	}
	if entry.Kind == NormalBoss && entry.Payment != Shared {
		return fmt.Errorf("%w: normal boss admission uses shared action points", ErrInvalid)
	}
	if _, used := f.usedIDs[entry.ID]; used {
		return ErrDuplicate
	}

	seen := make(map[PlayerID]struct{}, len(entry.Players))
	for _, player := range entry.Players {
		balance, exists := f.personal[player]
		if !exists {
			return fmt.Errorf("%w: %q", ErrUnknownPlayer, player)
		}
		if _, duplicate := seen[player]; duplicate {
			return fmt.Errorf("%w: duplicate participant %q", ErrInvalid, player)
		}
		seen[player] = struct{}{}
		if _, busy := f.busy[player]; busy {
			return fmt.Errorf("%w: %q", ErrPlayerBusy, player)
		}
		if entry.Payment == Personal && balance < entry.Cost {
			return fmt.Errorf("%w: player %q", ErrInsufficient, player)
		}
	}

	if entry.Payment == Shared {
		if f.shared < entry.Cost {
			return fmt.Errorf("%w: shared pool", ErrInsufficient)
		}
		if f.authorizer == nil || entry.DecisionID == "" {
			return ErrApprovalRequired
		}
		if _, used := f.usedDecisions[entry.DecisionID]; used {
			return ErrDuplicate
		}
		spend := SharedSpend{FloorID: f.id, DecisionID: entry.DecisionID,
			EncounterID: entry.ID, Kind: entry.Kind, Cost: entry.Cost}
		if err := f.authorizer.Authorize(spend); err != nil {
			return fmt.Errorf("%w: %w", ErrApprovalDenied, err)
		}
	}

	// Nothing below this point can fail: debit, admission, and route selection
	// become visible together, with no observable intermediate exhausted state.
	if entry.Payment == Shared {
		f.shared -= entry.Cost
		f.usedDecisions[entry.DecisionID] = struct{}{}
	} else {
		for _, player := range entry.Players {
			f.personal[player] -= entry.Cost
		}
	}
	f.active[entry.ID] = Encounter{Kind: entry.Kind, Players: append([]PlayerID(nil), entry.Players...)}
	f.usedIDs[entry.ID] = struct{}{}
	for _, player := range entry.Players {
		f.busy[player] = entry.ID
	}
	if entry.Kind == NormalBoss {
		f.bossRoute, f.bossEncounter = NormalBossRoute, entry.ID
	}
	return nil
}

// CompleteEncounter is called after the server settles the encounter. A
// repeated completion is rejected, so downstream code can avoid reapplying
// rewards. Reward persistence and combat outcomes belong to the coordinator.
func (f *Floor) CompleteEncounter(id EncounterID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	encounter, exists := f.active[id]
	if !exists {
		return ErrEncounterNotFound
	}
	for _, player := range encounter.Players {
		delete(f.busy, player)
	}
	delete(f.active, id)
	return nil
}

// CommitEnhancedBoss records routing only. The caller handles assembly and
// starting combat. Exhaustion alone is insufficient while nodes still settle.
func (f *Floor) CommitEnhancedBoss(id EncounterID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bossRoute != NoBossRoute {
		return ErrBossCommitted
	}
	if id == "" {
		return ErrInvalid
	}
	if _, used := f.usedIDs[id]; used {
		return ErrDuplicate
	}
	if f.phase() != EnhancedBossReady {
		return ErrEnhancedNotReady
	}
	f.bossRoute, f.bossEncounter = EnhancedBossRoute, id
	f.usedIDs[id] = struct{}{}
	return nil
}

func (f *Floor) Snapshot() Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := Snapshot{FloorID: f.id, Personal: make(map[PlayerID]int64, len(f.personal)),
		Shared: f.shared, Active: make(map[EncounterID]Encounter, len(f.active)),
		Phase: f.phase(), BossRoute: f.bossRoute, BossEncounter: f.bossEncounter}
	for id, value := range f.personal {
		s.Personal[id] = value
	}
	for id, encounter := range f.active {
		s.Active[id] = Encounter{Kind: encounter.Kind, Players: append([]PlayerID(nil), encounter.Players...)}
	}
	return s
}

// phase requires the floor lock. Check balances individually, avoiding overflow
// from summing configurable values across an arbitrarily sized team.
func (f *Floor) phase() Phase {
	if f.bossRoute != NoBossRoute {
		return BossCommitted
	}
	if f.shared > 0 {
		return Exploring
	}
	for _, balance := range f.personal {
		if balance > 0 {
			return Exploring
		}
	}
	if len(f.active) > 0 {
		return ExhaustedSettling
	}
	return EnhancedBossReady
}
