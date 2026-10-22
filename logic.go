package main

import "strings"

func activeSelectionColor() uint32 {
	const r, g, b = uint32(33), uint32(97), uint32(239)
	return r | g<<8 | b<<16
}

// session is the platform-independent data shown for one Windows logon session.
type session struct {
	id    uint32
	name  string
	state string
}

type selectionState struct {
	ids     map[uint32]bool
	anchor  uint32
	current uint32
}

func newSelectionState() selectionState {
	return selectionState{ids: make(map[uint32]bool)}
}

func (s *selectionState) ensure() {
	if s.ids == nil {
		s.ids = make(map[uint32]bool)
	}
}

func (s *selectionState) Clear() {
	s.ids = make(map[uint32]bool)
	s.anchor = 0
	s.current = 0
}

func (s *selectionState) Contains(id uint32) bool { return s.ids[id] }
func (s *selectionState) Len() int                { return len(s.ids) }

// Click applies the same selection rules as Ctrl/Shift-click in the list.
func (s *selectionState) Click(visible []session, index int, ctrl, shift bool) bool {
	if index < 0 || index >= len(visible) {
		return false
	}
	s.ensure()
	id := visible[index].id
	if shift && s.anchor != 0 {
		start := sessionIndex(visible, s.anchor)
		if start < 0 {
			start = index
			s.anchor = id
		}
		if !ctrl {
			s.ids = make(map[uint32]bool)
		}
		lo, hi := start, index
		if lo > hi {
			lo, hi = hi, lo
		}
		for i := lo; i <= hi; i++ {
			s.ids[visible[i].id] = true
		}
		s.current = id
		return true
	}
	if ctrl {
		if s.ids[id] {
			delete(s.ids, id)
		} else {
			s.ids[id] = true
		}
		if s.anchor == 0 {
			s.anchor = id
		}
		s.current = id
		return true
	}
	s.Clear()
	s.ids[id] = true
	s.anchor = id
	s.current = id
	return true
}

// SelectVisible adds all current filter results without dropping hidden selections.
func (s *selectionState) SelectVisible(visible []session) {
	s.ensure()
	for i, item := range visible {
		s.ids[item.id] = true
		if i == 0 {
			s.anchor = item.id
			s.current = item.id
		}
	}
}

func (s *selectionState) ExtendTo(visible []session, index int) bool {
	if index < 0 || index >= len(visible) {
		return false
	}
	s.ensure()
	id := visible[index].id
	if s.anchor == 0 {
		s.anchor = id
	}
	s.current = id
	s.ids[id] = true
	return true
}

func (s *selectionState) RetainAvailable(all []session) {
	available := make(map[uint32]bool, len(all))
	for _, item := range all {
		available[item.id] = true
	}
	for id := range s.ids {
		if !available[id] {
			delete(s.ids, id)
		}
	}
	if !available[s.anchor] {
		s.anchor = 0
	}
	if !available[s.current] {
		s.current = 0
	}
}

func (s *selectionState) Sessions(all []session) []session {
	chosen := make([]session, 0, len(s.ids))
	for _, item := range all {
		if s.ids[item.id] {
			chosen = append(chosen, item)
		}
	}
	return chosen
}

func FilterSessions(all []session, query string) []session {
	needle := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]session, 0, len(all))
	for _, item := range all {
		if needle == "" || strings.Contains(strings.ToLower(item.name), needle) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func sessionIndex(items []session, id uint32) int {
	for i, item := range items {
		if item.id == id {
			return i
		}
	}
	return -1
}

func adjacentIndex(visible []session, currentID uint32, firstVisible int, down bool) int {
	if len(visible) == 0 {
		return -1
	}
	index := sessionIndex(visible, currentID)
	if index < 0 {
		index = firstVisible
		if index < 0 {
			index = 0
		}
	} else if down {
		index++
	} else {
		index--
	}
	if index < 0 {
		return 0
	}
	if index >= len(visible) {
		return len(visible) - 1
	}
	return index
}

func clampFirstVisible(first, total, visible int) int {
	if visible < 1 {
		visible = 1
	}
	maxFirst := total - visible
	if maxFirst < 0 {
		maxFirst = 0
	}
	if first < 0 {
		return 0
	}
	if first > maxFirst {
		return maxFirst
	}
	return first
}

func wheelFirstVisible(first, wheelDelta, total, visible, rowsPerWheel int) int {
	if rowsPerWheel < 1 {
		rowsPerWheel = 1
	}
	first -= (wheelDelta / 120) * rowsPerWheel
	return clampFirstVisible(first, total, visible)
}

func visibleRowsForLayout(listBottom, rowTop, rowHeight int) int {
	if rowHeight < 1 {
		return 1
	}
	rows := (listBottom - rowTop - 4) / rowHeight
	if rows < 1 {
		return 1
	}
	return rows
}

func queryWorkerCount(total, limit int) int {
	if total <= 0 || limit <= 0 {
		return 0
	}
	if total < limit {
		return total
	}
	return limit
}

func namedSessions(items []session) []session {
	users := make([]session, 0, len(items))
	for _, item := range items {
		if item.name != "" {
			users = append(users, item)
		}
	}
	return users
}

type keyAction uint8

const (
	keyActionNone keyAction = iota
	keyActionClearSelection
	keyActionSelectAll
	keyActionFocusSearch
	keyActionExtendUp
	keyActionExtendDown
)

func actionForKey(key uint16, ctrl, shift bool) keyAction {
	switch {
	case key == 0x1B: // Escape
		return keyActionClearSelection
	case ctrl && key == 0x41: // Ctrl+A
		return keyActionSelectAll
	case ctrl && key == 0x46: // Ctrl+F
		return keyActionFocusSearch
	case shift && key == 0x26: // Shift+Up
		return keyActionExtendUp
	case shift && key == 0x28: // Shift+Down
		return keyActionExtendDown
	default:
		return keyActionNone
	}
}
