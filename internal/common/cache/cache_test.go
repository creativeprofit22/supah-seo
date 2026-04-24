package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cache")
	return &FileStore{
		baseDir: dir,
		nowFunc: time.Now,
	}
}

func TestSetAndGet(t *testing.T) {
	store := newTestStore(t)

	payload, _ := json.Marshal(map[string]string{"result": "test"})
	rec := Record{
		Payload:    payload,
		Source:     "test-provider",
		FetchedAt:  time.Now().Format(time.RFC3339),
		TTLSeconds: 3600,
	}

	key := map[string]string{"query": "test"}

	if err := store.Set("test-provider", key, rec); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	got, hit, err := store.Get("test-provider", key)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !hit {
		t.Fatal("expected cache hit")
	}
	if got.Source != "test-provider" {
		t.Fatalf("expected source 'test-provider', got %q", got.Source)
	}
}

func TestGetMiss(t *testing.T) {
	store := newTestStore(t)
	_, hit, err := store.Get("test-provider", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hit {
		t.Fatal("expected cache miss")
	}
}

func TestExpiredEntry(t *testing.T) {
	store := newTestStore(t)

	pastTime := time.Now().Add(-2 * time.Hour)
	payload, _ := json.Marshal("expired")
	rec := Record{
		Payload:    payload,
		Source:     "test",
		FetchedAt:  pastTime.Format(time.RFC3339),
		TTLSeconds: 3600,
	}

	key := "expire-test"
	if err := store.Set("test", key, rec); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	_, hit, err := store.Get("test", key)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if hit {
		t.Fatal("expected expired entry to be a miss")
	}
}

func TestFetchedAtAutoSet(t *testing.T) {
	store := newTestStore(t)

	payload, _ := json.Marshal("auto")
	rec := Record{
		Payload:    payload,
		Source:     "test",
		TTLSeconds: 3600,
	}

	key := "auto-fetch"
	if err := store.Set("test", key, rec); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	got, hit, err := store.Get("test", key)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if !hit {
		t.Fatal("expected cache hit")
	}
	if got.FetchedAt == "" {
		t.Fatal("expected fetched_at to be auto-set")
	}
}

func TestCacheFilePermissions(t *testing.T) {
	// Skip when the underlying filesystem does not enforce Unix permissions
	// (e.g. WSL with a DrvFs mount, or a Windows volume mounted in Linux).
	// Create a probe file, chmod to 0o600, and confirm it reads back. If not,
	// the test has nothing meaningful to assert on this mount.
	probe := filepath.Join(t.TempDir(), "perm-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o600); err != nil {
		t.Fatalf("probe write failed: %v", err)
	}
	if pi, err := os.Stat(probe); err != nil {
		t.Fatalf("probe stat failed: %v", err)
	} else if pi.Mode().Perm() != 0o600 {
		t.Skipf("filesystem does not enforce Unix permissions (got %o); skipping", pi.Mode().Perm())
	}

	store := newTestStore(t)

	payload, _ := json.Marshal("perm-test")
	rec := Record{
		Payload:    payload,
		Source:     "test",
		TTLSeconds: 3600,
	}

	key := "perm-check"
	if err := store.Set("test", key, rec); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	path, err := store.recordPath("test", key)
	if err != nil {
		t.Fatalf("recordPath failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Fatalf("expected permissions 0600, got %o", perm)
	}
}
