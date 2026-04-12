package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/supah-seo/supah-seo/internal/common/config"
)

// Record stores cached payload plus freshness metadata.
type Record struct {
	Payload    json.RawMessage `json:"payload"`
	Source     string          `json:"source"`
	FetchedAt  string          `json:"fetched_at"`
	TTLSeconds int64           `json:"ttl_seconds"`
}

// FileStore persists cache records on disk.
type FileStore struct {
	baseDir string
	nowFunc func() time.Time
}

// NewFileStore creates a file-based cache under the Supah SEO config directory.
func NewFileStore() *FileStore {
	baseDir := filepath.Join(filepath.Dir(config.Path()), "cache")
	return &FileStore{
		baseDir: baseDir,
		nowFunc: time.Now,
	}
}

// Get loads a record when present and not expired.
func (s *FileStore) Get(provider string, request any) (Record, bool, error) {
	path, err := s.recordPath(provider, request)
	if err != nil {
		return Record{}, false, err
	}

	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("reading cache record: %w", err)
	}

	var rec Record
	if err := json.Unmarshal(body, &rec); err != nil {
		return Record{}, false, fmt.Errorf("decoding cache record: %w", err)
	}

	if rec.TTLSeconds > 0 && rec.FetchedAt != "" {
		fetchedAt, err := time.Parse(time.RFC3339, rec.FetchedAt)
		if err == nil {
			expiresAt := fetchedAt.Add(time.Duration(rec.TTLSeconds) * time.Second)
			if s.nowFunc().After(expiresAt) {
				_ = os.Remove(path)
				return Record{}, false, nil
			}
		}
	}

	return rec, true, nil
}

// Set persists a cache record for a provider request.
func (s *FileStore) Set(provider string, request any, record Record) error {
	path, err := s.recordPath(provider, request)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	if record.FetchedAt == "" {
		record.FetchedAt = s.nowFunc().Format(time.RFC3339)
	}

	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding cache record: %w", err)
	}

	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("writing cache record: %w", err)
	}

	return nil
}

func (s *FileStore) recordPath(provider string, request any) (string, error) {
	hash, err := requestHash(request)
	if err != nil {
		return "", err
	}

	providerName := strings.TrimSpace(strings.ToLower(provider))
	if providerName == "" {
		providerName = "unknown"
	}

	return filepath.Join(s.baseDir, providerName, hash+".json"), nil
}

func requestHash(request any) (string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encoding cache request: %w", err)
	}

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
