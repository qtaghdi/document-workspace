package xlsx

func (s *Session) EventsAfter(after uint64) ([]Event, Snapshot) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.replayGapLocked(after) {
		return []Event{s.resyncEventLocked()}, s.snapshotLocked()
	}
	events := make([]Event, 0)
	for _, event := range s.history {
		if event.Sequence > after {
			events = append(events, event)
		}
	}
	return events, s.snapshotLocked()
}

func (s *Session) Subscribe(after uint64) (<-chan Event, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	replay := make([]Event, 0)
	if s.replayGapLocked(after) {
		replay = append(replay, s.resyncEventLocked())
	} else {
		for _, event := range s.history {
			if event.Sequence > after {
				replay = append(replay, event)
			}
		}
	}
	ch := make(chan Event, len(replay)+64)
	for _, event := range replay {
		ch <- event
	}
	s.subscribers[ch] = struct{}{}
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}

func (s *Session) publishLocked(event Event) {
	s.sequence++
	event.Sequence = s.sequence
	s.history = append(s.history, event)
	if len(s.history) > maxEventHistory {
		s.history = append([]Event(nil), s.history[len(s.history)-maxEventHistory:]...)
	}
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// Never silently drop a commit. Disconnect so the client reconciles.
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}

func (s *Session) replayGapLocked(after uint64) bool {
	return after > s.sequence || (len(s.history) > 0 && after < s.history[0].Sequence-1)
}

func (s *Session) resyncEventLocked() Event {
	return Event{Sequence: s.sequence, Revision: s.revision, Actor: "system", Type: "workbook.reload", State: "resync"}
}
