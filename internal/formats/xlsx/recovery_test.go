package xlsx

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func recoverySession(t *testing.T) (*Session, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.xlsx")
	f := excelize.NewFile()
	if err := f.SetCellValue("Sheet1", "A1", "original"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

type failingFile struct {
	atomicFile
	stage string
}

var errInjected = errors.New("injected persistence failure")

func (f failingFile) Write(b []byte) (int, error) {
	if f.stage == "write" {
		return 0, errInjected
	}
	if f.stage == "short" {
		return len(b) - 1, nil
	}
	return f.atomicFile.Write(b)
}
func (f failingFile) Sync() error {
	if f.stage == "sync" {
		return errInjected
	}
	return f.atomicFile.Sync()
}
func (f failingFile) Close() error {
	err := f.atomicFile.Close()
	if f.stage == "close" {
		return errInjected
	}
	return err
}

func TestPersistenceFailuresPreserveSessionAndDisk(t *testing.T) {
	for _, restore := range []bool{false, true} {
		for _, stage := range []string{"create", "write", "short", "sync", "close", "rename", "history"} {
			name := stage
			if restore {
				name += "-undo"
			}
			t.Run(name, func(t *testing.T) {
				s, path := recoverySession(t)
				if restore {
					if _, err := s.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "saved"}}); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				snapshot := s.Snapshot()
				disk := diskPersistence()
				s.persistence.createTemp = func(dir, pattern string) (atomicFile, error) {
					if stage == "create" {
						return nil, errInjected
					}
					f, err := disk.createTemp(dir, pattern)
					if err != nil {
						return nil, err
					}
					return failingFile{f, stage}, nil
				}
				if stage == "rename" {
					s.persistence.rename = func(string, string) error { return errInjected }
				}
				if stage == "history" {
					if err := os.Mkdir(s.historyStore.pendingPath, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(s.historyStore.pendingPath, "blocker"), []byte("x"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if restore {
					_, err = s.RestoreHistory(snapshot.Revision, "human", "undo")
				} else {
					_, err = s.Apply(snapshot.Revision, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "unsaved"}})
				}
				if err == nil {
					t.Fatal("failure was accepted")
				}
				after, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("disk changed")
				}
				state := s.Snapshot()
				if state.Revision != snapshot.Revision || state.CanUndo != snapshot.CanUndo || state.CanRedo != snapshot.CanRedo {
					t.Fatal("history or revision changed")
				}
				value, err := s.ReadRange("Sheet1", "A1")
				if err != nil {
					t.Fatal(err)
				}
				expected := "original"
				if restore {
					expected = "saved"
				}
				if value.Rows[0][0].Value != expected {
					t.Fatalf("memory changed: %s", value.Rows[0][0].Value)
				}
				events, _ := s.EventsAfter(snapshot.Sequence)
				for _, event := range events {
					if event.Type != "presence.update" && event.Type != "cell.typing" {
						t.Fatalf("false commit: %#v", event)
					}
				}
				temps, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".document-workspace-*.tmp"))
				if len(temps) != 0 {
					t.Fatal("temporary files leaked")
				}
				s.persistence = diskPersistence()
				if stage == "history" {
					_ = os.Remove(filepath.Join(s.historyStore.pendingPath, "blocker"))
					_ = os.Remove(s.historyStore.pendingPath)
				}
				if _, err := s.Apply(snapshot.Revision, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "B1", Value: "retry"}}); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestExternalChangesBlockApplyAndRestore(t *testing.T) {
	for _, restore := range []bool{false, true} {
		s, path := recoverySession(t)
		if _, err := s.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "B1", Value: "saved"}}); err != nil {
			t.Fatal(err)
		}
		external := []byte("external replacement")
		if err := os.WriteFile(path, external, 0o600); err != nil {
			t.Fatal(err)
		}
		var err error
		if restore {
			_, err = s.RestoreHistory(2, "human", "undo")
		} else {
			_, err = s.Apply(2, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "overwrite"}})
		}
		if !errors.Is(err, ErrExternalChange) {
			t.Fatalf("expected external conflict: %v", err)
		}
		current, _ := os.ReadFile(path)
		if !bytes.Equal(current, external) || s.Snapshot().Revision != 2 {
			t.Fatal("external content or revision changed")
		}
	}
}

func TestLateExternalChangePreservesAuthoritativeMemory(t *testing.T) {
	s, path := recoverySession(t)
	disk := diskPersistence()
	s.persistence.createTemp = func(dir, pattern string) (atomicFile, error) {
		if err := os.WriteFile(path, []byte("external"), 0o600); err != nil {
			return nil, err
		}
		return disk.createTemp(dir, pattern)
	}
	_, err := s.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "candidate"}})
	if !errors.Is(err, ErrExternalChange) {
		t.Fatalf("late change: %v", err)
	}
	value, _ := s.ReadRange("Sheet1", "A1")
	if value.Rows[0][0].Value != "original" {
		t.Fatal("rollback imported external state")
	}
	if _, err := os.Stat(s.historyStore.pendingPath); !os.IsNotExist(err) {
		t.Fatal("pending transaction not aborted")
	}
}

func TestWriterLockAndStaleSession(t *testing.T) {
	s, path := recoverySession(t)
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	unlock, err := s.acquireWriteLock()
	if err != nil {
		t.Fatal(err)
	}
	_, err = second.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "blocked"}})
	if !errors.Is(err, ErrWriterBusy) {
		t.Fatalf("writer lock: %v", err)
	}
	unlock()
	if _, err := s.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "first"}}); err != nil {
		t.Fatal(err)
	}
	_, err = second.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "second"}})
	if !errors.Is(err, ErrExternalChange) {
		t.Fatalf("stale session: %v", err)
	}
}

func TestReplayGapsAndSlowSubscribersRequireResync(t *testing.T) {
	s, _ := recoverySession(t)
	ch, cancel := s.Subscribe(0)
	defer cancel()
	for i := 0; i < maxEventHistory+2; i++ {
		if err := s.UpdatePresence("ai", Presence{Sheet: "Sheet1", Range: "A1", State: "idle"}); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for range ch {
		count++
	}
	if count != 64 {
		t.Fatalf("slow subscriber buffer = %d", count)
	}
	for _, after := range []uint64{0, 1, s.sequence + 1} {
		events, _ := s.EventsAfter(after)
		if len(events) != 1 || events[0].State != "resync" {
			t.Fatalf("missing resync for %d: %#v", after, events)
		}
		stream, closeStream := s.Subscribe(after)
		if event := <-stream; event.State != "resync" {
			t.Fatalf("stream gap: %#v", event)
		}
		closeStream()
	}
	events, _ := s.EventsAfter(s.sequence - 1)
	if len(events) != 1 || events[0].Type != "presence.update" {
		t.Fatal("valid replay lost")
	}
}

func TestPendingHistoryRecoveryAfterManifestFailure(t *testing.T) {
	s, path := recoverySession(t)
	s.persistence.rename = func(from, to string) error {
		if err := os.Rename(from, to); err != nil {
			return err
		}
		// Inject the manifest failure only after the workbook is durable.
		if err := os.Mkdir(s.historyStore.manifestPath, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(s.historyStore.manifestPath, "blocker"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	snapshot, err := s.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "saved"}})
	if err != nil || snapshot.Revision != 2 {
		t.Fatalf("durable workbook must report success: %v", err)
	}
	pending, err := readHistoryManifest(s.historyStore.pendingPath)
	if err != nil || pending.Revision != 2 {
		t.Fatalf("pending recovery missing: %v", err)
	}
	// Another mutation must not overwrite unresolved recovery metadata.
	if _, err = s.Apply(2, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "lost"}}); err == nil {
		t.Fatal("unresolved history accepted")
	}
	_ = os.Remove(filepath.Join(s.historyStore.manifestPath, "blocker"))
	_ = os.Remove(s.historyStore.manifestPath)
	recovered, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if recovered.Snapshot().Revision != 2 || !recovered.Snapshot().CanUndo {
		t.Fatal("pending commit not recovered")
	}
}

func TestLostResponseCannotRepeatStructuralWrite(t *testing.T) {
	s, _ := recoverySession(t)
	operations := []Operation{{Type: "insert_rows", Sheet: "Sheet1", Index: 1, Count: 1}}
	if _, err := s.Apply(1, "ai", operations); err != nil {
		t.Fatal(err)
	}
	// Simulate a caller that lost the first response and repeats its old request.
	if _, err := s.Apply(1, "ai", operations); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("retry: %v", err)
	}
	value, err := s.ReadRange("Sheet1", "A2")
	if err != nil || value.Rows[0][0].Value != "original" || s.Snapshot().Revision != 2 {
		t.Fatal("structural write repeated")
	}
}

func TestRestoredIdenticalBytesStillRejectStaleSession(t *testing.T) {
	s, path := recoverySession(t)
	stale, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Close()
	if _, err = s.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "new"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestoreHistory(2, "human", "undo"); err != nil {
		t.Fatal(err)
	}
	if _, err = stale.Apply(1, "human", []Operation{{Type: "set_cell", Sheet: "Sheet1", Cell: "A1", Value: "stale"}}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("identical-byte stale session: %v", err)
	}
}
