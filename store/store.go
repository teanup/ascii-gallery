package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/teanup/ascii-gallery/model"
)

// Store provides thread-safe file-based persistence for animations.
type Store struct {
	dir string
	mu  sync.RWMutex
}

// New creates a Store backed by the given directory.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// path returns the file path for an animation ID.
func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// Save persists an animation to disk.
func (s *Store) Save(anim *model.Animation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(anim)
	if err != nil {
		return fmt.Errorf("marshal animation: %w", err)
	}

	if err := os.WriteFile(s.path(anim.ID), data, 0644); err != nil {
		return fmt.Errorf("write animation file: %w", err)
	}
	return nil
}

// Load reads an animation from disk.
func (s *Store) Load(id string) (*model.Animation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("animation %q not found", id)
		}
		return nil, fmt.Errorf("read animation file: %w", err)
	}

	var anim model.Animation
	if err := json.Unmarshal(data, &anim); err != nil {
		return nil, fmt.Errorf("unmarshal animation: %w", err)
	}
	return &anim, nil
}

// List returns all animation IDs sorted alphabetically.
func (s *Store) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read store directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
	}
	sort.Strings(ids)
	return ids, nil
}

// Delete removes an animation from disk.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path(id)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("animation %q not found", id)
		}
		return fmt.Errorf("delete animation file: %w", err)
	}
	return nil
}
