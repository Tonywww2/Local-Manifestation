package timing

import (
	"testing"
	"time"
)

func TestFixedStepsIndependentOfWakeFrequency(t *testing.T) {
	start := time.Unix(1, 0)
	for _, logic := range []int{30, 60} {
		for _, wakes := range []int{15, 30, 60, 120, 144} {
			s := NewStepper(logic, start)
			steps := 0
			for i := 1; i <= wakes*5; i++ {
				steps += s.Advance(start.Add(time.Duration(i) * 5 * time.Second / time.Duration(wakes*5)))
			}
			if steps != logic*5 || s.Dropped != 0 {
				t.Fatalf("logic=%d wakes=%d steps=%d dropped=%d", logic, wakes, steps, s.Dropped)
			}
		}
	}
}
func TestCatchUpCarriesDebtAndReportsOverload(t *testing.T) {
	start := time.Unix(1, 0)
	s := NewStepper(60, start)
	now := start.Add(time.Second / 2)
	steps := s.Advance(now)
	if steps != MaxCatchUp {
		t.Fatal(steps)
	}
	for {
		n := s.Advance(now)
		steps += n
		if n == 0 {
			break
		}
	}
	if steps != 30 || s.Dropped != 0 {
		t.Fatal(steps, s.Dropped)
	}
	if s.Advance(now.Add(10*time.Second)) != MaxCatchUp || s.Dropped == 0 {
		t.Fatal("unbounded overload")
	}
}
func TestPacingDoesNotBankFutureFrames(t *testing.T) {
	start := time.Unix(1, 0)
	p := Pacer{Rate: 120}
	p.Mark(start)
	if p.Delay(start) < 8*time.Millisecond {
		t.Fatal("no cap")
	}
	late := start.Add(time.Second)
	if p.Delay(late) != 0 {
		t.Fatal("waits after stall")
	}
	p.Mark(late)
	if p.Delay(late) < 8*time.Millisecond {
		t.Fatal("burst after stall")
	}
	p.Set(0)
	if p.Delay(late) != 0 {
		t.Fatal("uncapped waits")
	}
}
