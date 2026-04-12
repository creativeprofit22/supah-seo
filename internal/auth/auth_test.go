package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *FileTokenStore {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "auth")
	return &FileTokenStore{
		baseDir: dir,
		nowFunc: time.Now,
	}
}

func TestSaveAndLoad(t *testing.T) {
	store := newTestStore(t)

	token := TokenRecord{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		Scope:        "webmasters.readonly",
	}

	if err := store.Save("gsc", token); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := store.Load("gsc")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.AccessToken != token.AccessToken {
		t.Fatalf("expected access token %q, got %q", token.AccessToken, loaded.AccessToken)
	}
	if loaded.RefreshToken != token.RefreshToken {
		t.Fatalf("expected refresh token %q, got %q", token.RefreshToken, loaded.RefreshToken)
	}
}

func TestLoadNotAuthenticated(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent service")
	}
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)

	token := TokenRecord{AccessToken: "delete-me", TokenType: "Bearer"}
	if err := store.Save("gsc", token); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := store.Delete("gsc"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err := store.Load("gsc")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteNonexistent(t *testing.T) {
	store := newTestStore(t)
	if err := store.Delete("nonexistent"); err != nil {
		t.Fatalf("expected no error deleting nonexistent token: %v", err)
	}
}

func TestStatusAuthenticated(t *testing.T) {
	store := newTestStore(t)

	token := TokenRecord{
		AccessToken: "valid",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	}
	if err := store.Save("gsc", token); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	st, err := store.Status("gsc")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !st.Authenticated {
		t.Fatal("expected authenticated=true")
	}
	if st.Service != "gsc" {
		t.Fatalf("expected service 'gsc', got %q", st.Service)
	}
}

func TestStatusExpired(t *testing.T) {
	store := newTestStore(t)

	token := TokenRecord{
		AccessToken: "expired",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	}
	if err := store.Save("gsc", token); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	st, err := store.Status("gsc")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if st.Authenticated {
		t.Fatal("expected authenticated=false for expired token")
	}
}

func TestStatusNotAuthenticated(t *testing.T) {
	store := newTestStore(t)

	st, err := store.Status("gsc")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if st.Authenticated {
		t.Fatal("expected authenticated=false")
	}
}
