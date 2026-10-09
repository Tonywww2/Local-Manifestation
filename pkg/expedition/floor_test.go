package expedition

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
)

type authorizeFunc func(SharedSpend) error

func (fn authorizeFunc) Authorize(spend SharedSpend) error { return fn(spend) }

// Tests that do not concern voting stand in for an already-approved decision.
// Production code must provide an actual server-side decision verifier.
var approved = authorizeFunc(func(SharedSpend) error { return nil })

func newTestFloor(t *testing.T, personal, shared int64) *Floor {
	t.Helper()
	f, err := NewFloor(Config{ID: "floor-1", Players: []PlayerID{"a", "b", "c"},
		PersonalInitial: personal, SharedInitial: shared, Authorizer: approved})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func enter(t *testing.T, f *Floor, entry Entry) {
	t.Helper()
	if err := f.Enter(entry); err != nil {
		t.Fatal(err)
	}
}

func finish(t *testing.T, f *Floor, id EncounterID) {
	t.Helper()
	if err := f.CompleteEncounter(id); err != nil {
		t.Fatal(err)
	}
}

func TestSharedExhaustionStillAllowsIndependentExploration(t *testing.T) {
	f := newTestFloor(t, 2, 1)
	enter(t, f, Entry{ID: "shared", Kind: Ordinary, Players: []PlayerID{"a", "b"},
		Payment: Shared, Cost: 1, DecisionID: "approved-opening"})
	// C opted out of the shared encounter and explores another room at once.
	enter(t, f, Entry{ID: "solo", Kind: Ordinary, Players: []PlayerID{"c"}, Payment: Personal, Cost: 1})
	s := f.Snapshot()
	if s.Shared != 0 || s.Personal["a"] != 2 || s.Personal["b"] != 2 || s.Personal["c"] != 1 || len(s.Active) != 2 || s.Phase != Exploring {
		t.Fatalf("unexpected split-party state: %+v", s)
	}
	finish(t, f, "shared")
	before := f.Snapshot()
	err := f.Enter(Entry{ID: "boss", Kind: NormalBoss, Players: []PlayerID{"a", "b"}, Payment: Shared, Cost: 1, DecisionID: "boss-vote"})
	if !errors.Is(err, ErrInsufficient) || !reflect.DeepEqual(before, f.Snapshot()) {
		t.Fatalf("normal boss must require shared funds, got %v", err)
	}
}

func TestPersonalCostIsChargedPerEntrant(t *testing.T) {
	f := newTestFloor(t, 3, 5)
	enter(t, f, Entry{ID: "duo", Kind: Ordinary, Players: []PlayerID{"a", "b"}, Payment: Personal, Cost: 1})
	s := f.Snapshot()
	if s.Personal["a"] != 2 || s.Personal["b"] != 2 || s.Personal["c"] != 3 || s.Shared != 5 {
		t.Fatalf("incorrect per-entrant payment: %+v", s)
	}
}

func TestExhaustionWaitsForEveryPaidEncounterToSettle(t *testing.T) {
	f := newTestFloor(t, 1, 0)
	enter(t, f, Entry{ID: "duo", Kind: Ordinary, Players: []PlayerID{"a", "b"}, Payment: Personal, Cost: 1})
	enter(t, f, Entry{ID: "solo", Kind: Ordinary, Players: []PlayerID{"c"}, Payment: Personal, Cost: 1})
	if got := f.Snapshot().Phase; got != ExhaustedSettling {
		t.Fatalf("phase = %q", got)
	}
	if err := f.CommitEnhancedBoss("boss"); !errors.Is(err, ErrEnhancedNotReady) {
		t.Fatalf("must wait for active rooms: %v", err)
	}
	finish(t, f, "duo")
	if got := f.Snapshot().Phase; got != ExhaustedSettling {
		t.Fatalf("must also wait for the solo room: %q", got)
	}
	finish(t, f, "solo")
	if got := f.Snapshot().Phase; got != EnhancedBossReady {
		t.Fatalf("phase = %q", got)
	}
	if err := f.CommitEnhancedBoss("boss"); err != nil {
		t.Fatal(err)
	}
	if s := f.Snapshot(); s.BossRoute != EnhancedBossRoute || s.Phase != BossCommitted {
		t.Fatalf("unexpected boss route: %+v", s)
	}
}

func TestLastSharedPointCanFundAnyNode(t *testing.T) {
	for _, kind := range []EncounterKind{Ordinary, NormalBoss} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			f := newTestFloor(t, 0, 1)
			enter(t, f, Entry{ID: "last", Kind: kind, Players: []PlayerID{"a", "b", "c"}, Payment: Shared, Cost: 1, DecisionID: "last-vote"})
			if f.Snapshot().Shared != 0 {
				t.Fatal("last point must be spent")
			}
			if kind == Ordinary && f.Snapshot().Phase != ExhaustedSettling {
				t.Fatal("ordinary node must finish before enhanced boss")
			}
			finish(t, f, "last")
			if kind == NormalBoss {
				if s := f.Snapshot(); s.BossRoute != NormalBossRoute || s.Phase != BossCommitted {
					t.Fatalf("normal boss was incorrectly upgraded: %+v", s)
				}
				if err := f.CommitEnhancedBoss("enhanced"); !errors.Is(err, ErrBossCommitted) {
					t.Fatalf("committed normal boss must remain normal: %v", err)
				}
			} else if f.Snapshot().Phase != EnhancedBossReady {
				t.Fatal("settled ordinary node must allow enhanced boss")
			}
		})
	}
}

func TestSharedSpendRequiresAuthorizationOfExactProposal(t *testing.T) {
	want := SharedSpend{FloorID: "floor-1", DecisionID: "vote-1", EncounterID: "node-1", Kind: Ordinary, Cost: 1}
	f, err := NewFloor(Config{ID: "floor-1", Players: []PlayerID{"a", "b", "c"}, PersonalInitial: 1, SharedInitial: 3,
		Authorizer: authorizeFunc(func(got SharedSpend) error {
			if got != want {
				return errors.New("proposal differs from the team decision")
			}
			return nil
		})})
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{ID: "node-1", Kind: Ordinary, Players: []PlayerID{"a", "b"}, Payment: Shared, Cost: 2, DecisionID: "vote-1"}
	before := f.Snapshot()
	if err := f.Enter(entry); !errors.Is(err, ErrApprovalDenied) || !reflect.DeepEqual(before, f.Snapshot()) {
		t.Fatalf("unapproved amount changed state: %v", err)
	}
	entry.Cost = 1
	enter(t, f, entry)
	finish(t, f, entry.ID)
	entry.ID = "other-node"
	if err := f.Enter(entry); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("decision must not be spent twice: %v", err)
	}
}

func TestNoDefaultSharedDecisionPolicy(t *testing.T) {
	f, err := NewFloor(Config{ID: "f", Players: []PlayerID{"a", "b"}, SharedInitial: 1})
	if err != nil {
		t.Fatal(err)
	}
	before := f.Snapshot()
	err = f.Enter(Entry{ID: "n", Kind: Ordinary, Players: []PlayerID{"a"}, Payment: Shared, Cost: 1, DecisionID: "claimed-vote"})
	if !errors.Is(err, ErrApprovalRequired) || !reflect.DeepEqual(before, f.Snapshot()) {
		t.Fatalf("missing verifier must not authorize spending: %v", err)
	}
}

func TestInvalidGroupAdmissionIsAtomic(t *testing.T) {
	tests := []struct {
		name    string
		players []PlayerID
		cost    int64
		want    error
	}{
		{"unknown member", []PlayerID{"a", "missing"}, 1, ErrUnknownPlayer},
		{"duplicate member", []PlayerID{"a", "a"}, 1, ErrInvalid},
		{"insufficient later member", []PlayerID{"a", "c"}, 2, ErrInsufficient},
		{"negative cost", []PlayerID{"a"}, -1, ErrInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newTestFloor(t, 2, 4)
			enter(t, f, Entry{ID: "previous", Kind: Ordinary, Players: []PlayerID{"c"}, Payment: Personal, Cost: 1})
			finish(t, f, "previous")
			before := f.Snapshot()
			err := f.Enter(Entry{ID: "attempt", Kind: Ordinary, Players: test.players, Payment: Personal, Cost: test.cost})
			if !errors.Is(err, test.want) || !reflect.DeepEqual(before, f.Snapshot()) {
				t.Fatalf("failed admission changed state or returned wrong error: %v", err)
			}
		})
	}
}

func TestConcurrentSharedSpendingCannotOverdraw(t *testing.T) {
	f := newTestFloor(t, 1, 1)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, player := range []PlayerID{"a", "b"} {
		go func(player PlayerID) {
			<-start
			results <- f.Enter(Entry{ID: EncounterID(player), Kind: Ordinary, Players: []PlayerID{player}, Payment: Shared, Cost: 1, DecisionID: string(player)})
		}(player)
	}
	close(start)
	wins, denied := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			wins++
		} else if errors.Is(err, ErrInsufficient) {
			denied++
		} else {
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if s := f.Snapshot(); wins != 1 || denied != 1 || s.Shared != 0 || len(s.Active) != 1 {
		t.Fatalf("invalid concurrent spending: wins=%d denied=%d state=%+v", wins, denied, s)
	}
}

func TestRetriesAndOverlappingParticipationDoNotChargeAgain(t *testing.T) {
	f := newTestFloor(t, 3, 4)
	entry := Entry{ID: "one", Kind: Ordinary, Players: []PlayerID{"a"}, Payment: Personal, Cost: 1}
	enter(t, f, entry)
	before := f.Snapshot()
	if err := f.Enter(entry); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("retry = %v", err)
	}
	entry.ID = "two"
	if err := f.Enter(entry); !errors.Is(err, ErrPlayerBusy) {
		t.Fatalf("overlapping participation = %v", err)
	}
	if !reflect.DeepEqual(before, f.Snapshot()) {
		t.Fatal("rejected entries changed balances")
	}
	finish(t, f, "one")
	if err := f.CompleteEncounter("one"); !errors.Is(err, ErrEncounterNotFound) {
		t.Fatalf("duplicate settlement = %v", err)
	}
	entry.ID = "one"
	if err := f.Enter(entry); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("completed visit ID reused = %v", err)
	}
}

func TestIndependentLayerInitialValuesAndConfigurablePlayerCount(t *testing.T) {
	for _, count := range []int{2, 8, 12} {
		players := make([]PlayerID, count)
		for i := range players {
			players[i] = PlayerID(fmt.Sprint(i))
		}
		first, err := NewFloor(Config{ID: "first", Players: players, PersonalInitial: 2, SharedInitial: 5})
		if err != nil {
			t.Fatal(err)
		}
		// Layer and team modifiers have been resolved by the caller to 4 and 7.
		next, err := NewFloor(Config{ID: "next", Players: players, PersonalInitial: 4, SharedInitial: 7})
		if err != nil {
			t.Fatal(err)
		}
		for _, pair := range []struct {
			floor *Floor
			x, y  int64
		}{{first, 2, 5}, {next, 4, 7}} {
			s := pair.floor.Snapshot()
			if len(s.Personal) != count || s.Shared != pair.y {
				t.Fatalf("incorrect layer initialization: %+v", s)
			}
			for _, points := range s.Personal {
				if points != pair.x {
					t.Fatal("players must have equal personal initial values")
				}
			}
		}
	}
}

func TestSnapshotAndInputCannotMutateFloor(t *testing.T) {
	f := newTestFloor(t, math.MaxInt64, 0)
	players := []PlayerID{"a"}
	enter(t, f, Entry{ID: "one", Kind: Ordinary, Players: players, Payment: Personal, Cost: 1})
	players[0] = "b"
	s := f.Snapshot()
	s.Personal["a"] = 0
	s.Active["one"].Players[0] = "c"
	delete(s.Active, "one")
	actual := f.Snapshot()
	if actual.Personal["a"] != math.MaxInt64-1 || actual.Active["one"].Players[0] != "a" || actual.Phase != Exploring {
		t.Fatalf("caller mutated floor or large values overflowed: %+v", actual)
	}
}
