package xlsx

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var ErrExternalChange = errors.New("workbook changed outside this session; reopen the workbook before editing")
var ErrWriterBusy = errors.New("workbook writer lock is held; another writer or interrupted save requires attention")

type atomicFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

type persistenceIO struct {
	createTemp func(string, string) (atomicFile, error)
	rename     func(string, string) error
}

func diskPersistence() persistenceIO {
	return persistenceIO{
		createTemp: func(dir, pattern string) (atomicFile, error) { return os.CreateTemp(dir, pattern) },
		rename:     os.Rename,
	}
}

// The sidecar lock coordinates cooperating processes across atomic replacement.
// A crash leaves a lock that fails closed; never automatically steal that lock.
func (s *Session) acquireWriteLock() (func(), error) {
	if err := os.MkdirAll(s.historyStore.dir, 0o700); err != nil {
		return nil, fmt.Errorf("create writer directory: %w", err)
	}
	lock := filepath.Join(s.historyStore.dir, "write.lock")
	if err := os.Mkdir(lock, 0o700); err != nil {
		if os.IsExist(err) {
			return nil, ErrWriterBusy
		}
		return nil, fmt.Errorf("acquire writer lock: %w", err)
	}
	return func() { _ = os.Remove(lock) }, nil
}

func (s *Session) readCurrentLocked() ([]byte, error) {
	info, err := os.Lstat(s.path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxWorkbookBytes {
		return nil, ErrExternalChange
	}
	content, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read current workbook: %w", err)
	}
	if hashBytes(content) != s.currentHash {
		return nil, ErrExternalChange
	}
	return content, nil
}

func (s *Session) serializeLocked() ([]byte, error) {
	buffer, err := s.file.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("serialize workbook: %w", err)
	}
	return buffer.Bytes(), nil
}

func (s *Session) reconcileHistoryLocked() error {
	if err := s.historyStore.reconcilePending(s.currentHash); err != nil {
		return err
	}
	manifest, err := readHistoryManifest(s.historyStore.manifestPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("verify current history revision: %w", err)
	}
	// Undo can restore identical bytes in another process. Content hashes alone
	// cannot establish that this session still owns the current revision.
	if manifest.Version == historyManifestVersion && manifest.CurrentHash == s.currentHash && manifest.Revision != s.revision {
		return fmt.Errorf("%w: another session advanced to %d; reopen the workbook", ErrRevisionConflict, manifest.Revision)
	}
	return nil
}

func (s *Session) replaceBytesLocked(content []byte) error {
	if err := validateWorkbookBytes(content, s.packageLimits); err != nil {
		return fmt.Errorf("validate workbook candidate: %w", err)
	}
	tmp, err := s.persistence.createTemp(filepath.Dir(s.path), ".document-workspace-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary workbook: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	info, err := os.Stat(s.path)
	if err == nil {
		err = tmp.Chmod(info.Mode())
	}
	if err == nil {
		var n int
		n, err = tmp.Write(content)
		if err == nil && n != len(content) {
			err = io.ErrShortWrite
		}
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write temporary workbook: %w", err)
	}
	// Check again after the potentially slow serialization/write. Non-cooperating
	// applications can still race the final check and rename; exclusive editing
	// is required when using Excel or another external writer.
	if _, err := s.readCurrentLocked(); err != nil {
		return err
	}
	if err := s.persistence.rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace workbook atomically: %w", err)
	}
	s.currentHash = hashBytes(content)
	return nil
}

func randomID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "workbook"
	}
	return hex.EncodeToString(value[:])
}
