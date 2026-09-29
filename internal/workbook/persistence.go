package workbook

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func (s *Session) persistLocked() error {
	buffer, err := s.file.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("serialize workbook: %w", err)
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".xlsx-viewer-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary workbook: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if info, statErr := os.Stat(s.path); statErr == nil {
		_ = tmp.Chmod(info.Mode())
	}
	if _, err = tmp.Write(buffer.Bytes()); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write temporary workbook: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace workbook atomically: %w", err)
	}
	return nil
}

func (s *Session) reloadLocked() {
	reloaded, err := excelize.OpenFile(s.path)
	if err != nil {
		return
	}
	_ = s.file.Close()
	s.file = reloaded
}

func randomID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "workbook"
	}
	return hex.EncodeToString(value[:])
}
