// Package timing separates fixed simulation steps from network and render pacing.
package timing

import "time"

const (
	DefaultLogicRate    = 30
	DefaultSnapshotRate = 20
	ClientRate          = 60
	MaxCatchUp          = 8
	maxBacklog          = 2 * time.Second
)

func LogicRate(rate int) bool { return rate == 30 || rate == 60 }

// Stepper retains fractional time and bounded catch-up debt. Each returned step
// must still use dt=1/Rate; overload never makes a simulation step larger.
type Stepper struct {
	Rate    int
	Dropped uint64
	last    time.Time
	pending time.Duration
}

func NewStepper(rate int, now time.Time) *Stepper {
	if !LogicRate(rate) {
		panic("invalid logic rate")
	}
	return &Stepper{Rate: rate, last: now}
}

func (s *Stepper) Advance(now time.Time) int {
	elapsed := now.Sub(s.last)
	if elapsed < 0 {
		return 0
	}
	s.last = now
	period := time.Second / time.Duration(s.Rate)
	if elapsed > maxBacklog {
		s.Dropped += uint64((elapsed - maxBacklog) / period)
		elapsed = maxBacklog
	}
	s.pending += elapsed
	if s.pending > maxBacklog {
		s.Dropped += uint64((s.pending - maxBacklog) / period)
		s.pending = maxBacklog
	}
	n := min(int(s.pending/period), MaxCatchUp)
	s.pending -= time.Duration(n) * period
	return n
}

// Meter measures completed work, rather than reporting a configured target.
type Meter struct {
	start time.Time
	count int
	rate  float64
}

func (m *Meter) Add(n int, now time.Time) {
	if m.start.IsZero() {
		m.start = now
		return
	}
	m.count += n
	if elapsed := now.Sub(m.start); elapsed >= time.Second {
		m.rate = float64(m.count) / elapsed.Seconds()
		m.count = 0
		m.start = now
	}
}
func (m *Meter) Rate() float64 { return m.rate }

// Pacer caps real frame draws. It never changes simulation TPS or repeats a
// cached image to count it as a newly rendered frame. Zero means uncapped.
type Pacer struct {
	Rate int
	next time.Time
}

func (p *Pacer) Set(rate int) { p.Rate = rate; p.next = time.Time{} }
func (p *Pacer) Delay(now time.Time) time.Duration {
	if p.Rate <= 0 {
		return 0
	}
	return max(time.Duration(0), p.next.Sub(now))
}
func (p *Pacer) Mark(now time.Time) {
	if p.Rate > 0 {
		p.next = now.Add(time.Second / time.Duration(p.Rate))
	}
}
func (p *Pacer) Wait() time.Time {
	now := time.Now()
	if delay := p.Delay(now); delay > 0 {
		time.Sleep(delay)
		now = time.Now()
	}
	p.Mark(now)
	return now
}
