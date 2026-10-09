package content

import (
	"fmt"
	"io/fs"
	"sync"
)

// Store publishes complete catalogs and pins one generation for an adventure.
// A preparation floor remains part of that adventure until its lease is released.
type Store struct {
	mu       sync.Mutex
	current  *Catalog
	active   int
	revision uint64
}

func NewStore(c *Catalog) *Store   { return &Store{current: c, revision: 1} }
func (s *Store) Current() *Catalog { s.mu.Lock(); defer s.mu.Unlock(); return s.current }
func (s *Store) Acquire() (*Catalog, func()) {
	s.mu.Lock()
	s.active++
	c := s.current
	s.mu.Unlock()
	var once sync.Once
	return c, func() { once.Do(func() { s.mu.Lock(); s.active--; s.mu.Unlock() }) }
}
func (s *Store) Reload(files fs.FS) error {
	s.mu.Lock()
	if s.active > 0 {
		s.mu.Unlock()
		return fmt.Errorf("content reload forbidden during an adventure")
	}
	if s.current.ContainsGo() {
		s.mu.Unlock()
		return fmt.Errorf("Go content requires restart")
	}
	revision := s.revision
	s.mu.Unlock()
	next, err := LoadWithOptions(files, LoadOptions{Previous: s.Current()})
	if err != nil {
		return err
	}
	if next.ContainsGo() {
		return fmt.Errorf("Go content requires restart")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active > 0 || s.revision != revision {
		return fmt.Errorf("content generation changed during reload")
	}
	s.current = next
	s.revision++
	return nil
}
func (s *Store) Publish(next *Catalog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active > 0 {
		return fmt.Errorf("content reload forbidden during an adventure")
	}
	if s.current.ContainsGo() || next.ContainsGo() {
		return fmt.Errorf("Go content requires restart")
	}
	s.current = next
	s.revision++
	return nil
}
