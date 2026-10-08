package xlsx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const historyManifestVersion = 1
const maxHistoryManifestBytes = 64 << 10

type historyStore struct {
	dir          string
	manifestPath string
	pendingPath  string
}

type historyManifest struct {
	Version     int      `json:"version"`
	Revision    uint64   `json:"revision"`
	CurrentHash string   `json:"currentHash"`
	Undo        []string `json:"undo,omitempty"`
	Redo        []string `json:"redo,omitempty"`
}

// loadHistoryStore restores revision history only when the sidecar manifest
// matches the current workbook bytes. An externally replaced workbook starts a
// fresh history instead of applying snapshots from a different file version.
func loadHistoryStore(workbookPath string, current []byte) (*historyStore, uint64, [][]byte, [][]byte, error) {
	key := sha256.Sum256([]byte(workbookPath))
	// Keep the original sidecar namespace so product upgrades retain undo history.
	dir := filepath.Join(filepath.Dir(workbookPath), ".xlsx-viewer-history", hex.EncodeToString(key[:8]))
	store := &historyStore{
		dir:          dir,
		manifestPath: filepath.Join(dir, "manifest.json"),
		pendingPath:  filepath.Join(dir, "manifest.pending.json"),
	}
	guard := &Session{historyStore: store}
	unlock, lockErr := guard.acquireWriteLock()
	if lockErr != nil {
		return nil, 0, nil, nil, lockErr
	}
	defer unlock()
	// Opening may have read the file before another process finished its save.
	// Never reconcile a pending manifest against a stale pre-lock snapshot.
	latest, readErr := os.ReadFile(workbookPath)
	if readErr != nil {
		return nil, 0, nil, nil, readErr
	}
	if hashBytes(latest) != hashBytes(current) {
		return nil, 0, nil, nil, ErrExternalChange
	}
	if err := store.reconcilePending(hashBytes(current)); err != nil {
		return nil, 0, nil, nil, err
	}
	manifest, err := readHistoryManifest(store.manifestPath)
	if os.IsNotExist(err) {
		return store, 1, nil, nil, nil
	}
	if err != nil {
		return store, 1, nil, nil, nil
	}
	if manifest.Version != historyManifestVersion || manifest.Revision < 1 || manifest.CurrentHash != hashBytes(current) ||
		len(manifest.Undo) > maxHistoryEntries || len(manifest.Redo) > maxHistoryEntries {
		return store, 1, nil, nil, nil
	}
	undo, err := store.readSnapshots(manifest.Undo)
	if err != nil {
		return store, 1, nil, nil, nil
	}
	redo, err := store.readSnapshots(manifest.Redo)
	if err != nil {
		return store, 1, nil, nil, nil
	}
	return store, manifest.Revision, undo, redo, nil
}

func (s *historyStore) prepare(revision uint64, current []byte, undo, redo [][]byte) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create durable history directory: %w", err)
	}
	manifest := historyManifest{
		Version:     historyManifestVersion,
		Revision:    revision,
		CurrentHash: hashBytes(current),
	}
	var err error
	if manifest.Undo, err = s.writeSnapshots(undo); err != nil {
		return err
	}
	if manifest.Redo, err = s.writeSnapshots(redo); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode durable history manifest: %w", err)
	}
	if err := writeAtomicFile(s.pendingPath, encoded, 0o600); err != nil {
		return fmt.Errorf("prepare durable history manifest: %w", err)
	}
	return nil
}

func (s *historyStore) commit() error {
	if err := os.Rename(s.pendingPath, s.manifestPath); err != nil {
		return fmt.Errorf("commit durable history manifest: %w", err)
	}
	manifest, err := readHistoryManifest(s.manifestPath)
	if err == nil {
		s.cleanupSnapshots(manifest)
	}
	return nil
}

func (s *historyStore) abort() {
	_ = os.Remove(s.pendingPath)
}

func (s *historyStore) reconcilePending(currentHash string) error {
	pending, err := readHistoryManifest(s.pendingPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || pending.Version != historyManifestVersion || pending.CurrentHash != currentHash {
		_ = os.Remove(s.pendingPath)
		return nil
	}
	if err := os.Rename(s.pendingPath, s.manifestPath); err != nil {
		return fmt.Errorf("recover durable history manifest: %w", err)
	}
	return nil
}

func (s *historyStore) writeSnapshots(snapshots [][]byte) ([]string, error) {
	hashes := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		hash := hashBytes(snapshot)
		path := filepath.Join(s.dir, hash+".xlsx")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := writeAtomicFile(path, snapshot, 0o600); err != nil {
				return nil, fmt.Errorf("write durable history snapshot: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("inspect durable history snapshot: %w", err)
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

func (s *historyStore) readSnapshots(hashes []string) ([][]byte, error) {
	snapshots := make([][]byte, 0, len(hashes))
	for _, hash := range hashes {
		if !validHistoryHash(hash) {
			return nil, fmt.Errorf("invalid durable history snapshot hash")
		}
		content, err := os.ReadFile(filepath.Join(s.dir, hash+".xlsx"))
		if err != nil {
			return nil, fmt.Errorf("read durable history snapshot: %w", err)
		}
		if len(content) > maxHistorySnapshotBytes || hashBytes(content) != hash {
			return nil, fmt.Errorf("invalid durable history snapshot content")
		}
		snapshots = append(snapshots, content)
	}
	return snapshots, nil
}

func (s *historyStore) cleanupSnapshots(manifest historyManifest) {
	referenced := make(map[string]struct{}, len(manifest.Undo)+len(manifest.Redo))
	for _, hash := range append(append([]string(nil), manifest.Undo...), manifest.Redo...) {
		referenced[hash+".xlsx"] = struct{}{}
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xlsx") {
			continue
		}
		if _, ok := referenced[entry.Name()]; !ok {
			_ = os.Remove(filepath.Join(s.dir, entry.Name()))
		}
	}
}

func readHistoryManifest(path string) (historyManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return historyManifest{}, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxHistoryManifestBytes+1))
	if err != nil {
		return historyManifest{}, err
	}
	if len(content) > maxHistoryManifestBytes {
		return historyManifest{}, fmt.Errorf("durable history manifest exceeds %d bytes", maxHistoryManifestBytes)
	}
	var manifest historyManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return historyManifest{}, fmt.Errorf("decode durable history manifest: %w", err)
	}
	return manifest, nil
}

func writeAtomicFile(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".xlsx-viewer-history-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(mode); err == nil {
		_, err = tmp.Write(content)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func validHistoryHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
