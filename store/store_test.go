package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teanup/ascii-gallery/model"
)

func TestStoreCRUD(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	anim := &model.Animation{
		ID:     "test",
		Name:   "Test",
		Width:  80,
		Height: 20,
		Delay:  100,
		Frames: []string{"frame1", "frame2"},
	}

	// Save and load.
	if err := s.Save(anim); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load("test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != anim.Name || got.Width != anim.Width || len(got.Frames) != 2 {
		t.Errorf("loaded animation mismatch: %+v", got)
	}

	// List.
	ids, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 || ids[0] != "test" {
		t.Errorf("List = %v, want [test]", ids)
	}

	// Delete.
	if err := s.Delete("test"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Load("test"); err == nil {
		t.Error("expected error loading deleted animation")
	}
	ids, _ = s.List()
	if len(ids) != 0 {
		t.Errorf("List after delete = %v, want empty", ids)
	}
}

func TestStoreLoadMissing(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := s.Load("missing"); err == nil {
		t.Error("expected error loading missing animation")
	}
}

func TestStoreDeleteMissing(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Delete("missing"); err == nil {
		t.Error("expected error deleting missing animation")
	}
}

func TestStoreListSorted(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, id := range []string{"zebra", "apple", "mango"} {
		if err := s.Save(&model.Animation{ID: id, Name: id}); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	ids, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"apple", "mango", "zebra"}
	if len(ids) != len(want) {
		t.Fatalf("List = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("List[%d] = %s, want %s", i, ids[i], want[i])
		}
	}
}

func TestStoreIgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Create a stray non-JSON file in the store directory.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write stray file: %v", err)
	}
	ids, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("List = %v, want empty (stray file ignored)", ids)
	}
}

func TestNewFailsOnFileDir(t *testing.T) {
	// A path that is a regular file cannot be used as the store directory.
	file := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := New(file); err == nil {
		t.Error("expected error when store dir is a file")
	}
}
