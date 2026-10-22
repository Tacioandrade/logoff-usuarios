package main

import (
	"encoding/binary"
	"image/png"
	"os"
	"reflect"
	"testing"
)

func testSessions(names ...string) []session {
	items := make([]session, len(names))
	for i, name := range names {
		items[i] = session{id: uint32(i + 1), name: name, state: "Ativa"}
	}
	return items
}

func selectedIDsOf(s selectionState) []uint32 {
	ids := make([]uint32, 0, len(s.ids))
	for id := range s.ids {
		ids = append(ids, id)
	}
	return ids
}

func TestFilterSessionsCaseInsensitiveSubstringAndTrim(t *testing.T) {
	all := testSessions(`DOMINIO\Ana.Silva`, `servidor\bruno`, `carla`)
	got := FilterSessions(all, "  aNa.sIL  ")
	if len(got) != 1 || got[0].id != 1 {
		t.Fatalf("FilterSessions() = %#v, want only Ana.Silva", got)
	}
	if got := FilterSessions(all, ""); !reflect.DeepEqual(got, all) {
		t.Fatalf("empty filter = %#v, want all sessions", got)
	}
	if got := FilterSessions(all, "missing"); len(got) != 0 {
		t.Fatalf("unmatched filter returned %d sessions, want 0", len(got))
	}
}

func TestSelectionClickSingleCtrlToggleAndShiftRange(t *testing.T) {
	items := testSessions("ana", "bruno", "carla", "diego")
	s := newSelectionState()
	s.Click(items, 0, false, false)
	s.Click(items, 2, true, false)
	if !reflect.DeepEqual(sortedIDs(selectedIDsOf(s)), []uint32{1, 3}) {
		t.Fatalf("Ctrl-click selection = %v, want [1 3]", sortedIDs(selectedIDsOf(s)))
	}
	s.Click(items, 3, false, true)
	if !reflect.DeepEqual(sortedIDs(selectedIDsOf(s)), []uint32{1, 2, 3, 4}) {
		t.Fatalf("Shift-click range = %v, want [1 2 3 4]", sortedIDs(selectedIDsOf(s)))
	}
	s.Click(items, 1, true, false)
	if s.Contains(2) {
		t.Fatal("Ctrl-click on selected user should toggle it off")
	}
	s.Click(items, 2, false, false)
	if !reflect.DeepEqual(selectedIDsOf(s), []uint32{3}) {
		t.Fatalf("plain click should leave one selection, got %v", selectedIDsOf(s))
	}
}

func TestSelectionShiftClickAfterFilterAndReverseCtrlRange(t *testing.T) {
	items := testSessions("ana", "bruno", "carla", "diego")
	s := newSelectionState()
	s.Click(items, 0, false, false)
	filtered := FilterSessions(items, "diego")
	s.Click(filtered, 0, false, true) // Anchor is hidden, so start at the clicked result.
	if !reflect.DeepEqual(selectedIDsOf(s), []uint32{4}) {
		t.Fatalf("filtered Shift-click should select the clicked result: %v", sortedIDs(selectedIDsOf(s)))
	}
	s.Click(items, 3, false, false)
	s.Click(items, 1, true, true) // Reverse range, retaining any existing selection.
	if !reflect.DeepEqual(sortedIDs(selectedIDsOf(s)), []uint32{2, 3, 4}) {
		t.Fatalf("Ctrl+Shift reverse range = %v, want [2 3 4]", sortedIDs(selectedIDsOf(s)))
	}
}

func TestFilterPreservesSelectionAndCtrlAAddsVisibleOnly(t *testing.T) {
	all := testSessions("ana", "bruno", "carla")
	s := newSelectionState()
	s.Click(all, 0, false, false)
	s.Click(all, 2, true, false)

	filtered := FilterSessions(all, "bruno")
	if !s.Contains(1) || !s.Contains(3) {
		t.Fatal("changing the filter must not clear selections hidden by the filter")
	}
	s.SelectVisible(filtered)
	if !reflect.DeepEqual(sortedIDs(selectedIDsOf(s)), []uint32{1, 2, 3}) {
		t.Fatalf("Ctrl+A should add visible rows and preserve hidden ones; got %v", sortedIDs(selectedIDsOf(s)))
	}
	chosen := s.Sessions(all)
	if len(chosen) != 3 || chosen[0].id != 1 || chosen[1].id != 2 || chosen[2].id != 3 {
		t.Fatalf("selected sessions should include hidden rows in source order, got %#v", chosen)
	}
}

func TestSelectionRetainsOnlySessionsStillPresentAfterRefresh(t *testing.T) {
	all := testSessions("ana", "bruno", "carla")
	s := newSelectionState()
	s.SelectVisible(all)
	s.RetainAvailable([]session{all[0], all[2]})
	if !reflect.DeepEqual(sortedIDs(selectedIDsOf(s)), []uint32{1, 3}) {
		t.Fatalf("selection after refresh = %v, want sessions still present [1 3]", sortedIDs(selectedIDsOf(s)))
	}
}

func TestShiftArrowSelectionExtendsAndClamps(t *testing.T) {
	items := testSessions("ana", "bruno", "carla")
	if got := adjacentIndex(items, 2, 0, true); got != 2 {
		t.Fatalf("Shift+Down adjacent index = %d, want 2", got)
	}
	if got := adjacentIndex(items, 1, 0, false); got != 0 {
		t.Fatalf("Shift+Up at first row = %d, want 0", got)
	}
	if got := adjacentIndex(items, 0, 1, true); got != 1 {
		t.Fatalf("Shift+Down after current row was filtered out = %d, want first visible row 1", got)
	}
	if got := adjacentIndex(items, 3, 0, true); got != len(items)-1 {
		t.Fatalf("Shift+Down at last row = %d, want %d", got, len(items)-1)
	}
	if got := adjacentIndex(items, 0, -1, true); got != 0 {
		t.Fatalf("missing current row and negative scroll offset = %d, want 0", got)
	}
	s := newSelectionState()
	s.Click(items, 0, false, false)
	s.ExtendTo(items, adjacentIndex(items, s.current, 0, true))
	if !reflect.DeepEqual(sortedIDs(selectedIDsOf(s)), []uint32{1, 2}) {
		t.Fatalf("Shift+Down should add the adjacent row, got %v", sortedIDs(selectedIDsOf(s)))
	}
}

func TestEscapeCtrlFAndSelectionKeyboardActions(t *testing.T) {
	cases := []struct {
		key         uint16
		ctrl, shift bool
		want        keyAction
	}{
		{0x1B, false, false, keyActionClearSelection},
		{0x41, true, false, keyActionSelectAll},
		{0x46, true, false, keyActionFocusSearch},
		{0x26, false, true, keyActionExtendUp},
		{0x28, false, true, keyActionExtendDown},
		{0x46, false, false, keyActionNone},
		{0x28, false, false, keyActionNone},
	}
	for _, tc := range cases {
		if got := actionForKey(tc.key, tc.ctrl, tc.shift); got != tc.want {
			t.Errorf("actionForKey(%#x, ctrl=%t, shift=%t) = %v, want %v", tc.key, tc.ctrl, tc.shift, got, tc.want)
		}
	}
	s := newSelectionState()
	s.Click(testSessions("ana"), 0, false, false)
	s.Clear()
	if s.Len() != 0 || s.anchor != 0 || s.current != 0 {
		t.Fatalf("Escape clear must reset selection and navigation state: %#v", s)
	}
}

func TestSelectedRowAvatarAndButtonShareHighContrastColor(t *testing.T) {
	const want = uint32(0x00ef6121) // COLORREF para #2161ef.
	if got := activeSelectionColor(); got != want {
		t.Fatalf("selection color = %#08x, want %#08x", got, want)
	}
}

func TestScrollBoundsWheelAndLargeSessionLists(t *testing.T) {
	if got := visibleRowsForLayout(300, 182, 56); got != 2 {
		t.Fatalf("visible row count = %d, want 2", got)
	}
	if got := visibleRowsForLayout(0, 182, 56); got != 1 {
		t.Fatalf("small window must still show at least one row, got %d", got)
	}
	if got := clampFirstVisible(-4, 36, 3); got != 0 {
		t.Fatalf("negative scroll offset = %d, want 0", got)
	}
	if got := clampFirstVisible(80, 36, 3); got != 33 {
		t.Fatalf("scroll offset past end = %d, want 33", got)
	}
	if got := wheelFirstVisible(10, -120, 36, 3, 3); got != 13 {
		t.Fatalf("one wheel notch down = %d, want 13", got)
	}
	if got := wheelFirstVisible(1, 120, 36, 3, 3); got != 0 {
		t.Fatalf("wheel up at top = %d, want 0", got)
	}
	if got := queryWorkerCount(36, 8); got != 8 {
		t.Fatalf("worker count for 36 sessions = %d, want 8", got)
	}
	if got := queryWorkerCount(3, 8); got != 3 {
		t.Fatalf("worker count for 3 sessions = %d, want 3", got)
	}
	users := make([]session, 36)
	for i := range users {
		users[i] = session{id: uint32(i + 1), name: "user"}
	}
	if got := len(namedSessions(users)); got != 36 {
		t.Fatalf("named session filtering truncated a large list to %d", got)
	}
}

func TestNamedSessionsDropsSessionsWithoutUsernames(t *testing.T) {
	got := namedSessions([]session{{id: 1, name: "ana"}, {id: 2}, {id: 3, name: "bruno"}})
	if len(got) != 2 || got[0].id != 1 || got[1].id != 3 {
		t.Fatalf("namedSessions() = %#v, want only named sessions", got)
	}
}

func TestSelectionAndPagingBoundaryCases(t *testing.T) {
	var zero selectionState
	zero.SelectVisible(nil)
	if zero.ids == nil || zero.Len() != 0 {
		t.Fatal("zero-value selection should initialize safely when selecting an empty list")
	}
	items := testSessions("ana", "bruno")
	if zero.Click(items, -1, false, false) || zero.Click(items, len(items), false, false) {
		t.Fatal("click outside the visible list must be ignored")
	}
	if zero.ExtendTo(items, len(items)) {
		t.Fatal("keyboard extension outside the visible list must be ignored")
	}
	if !zero.ExtendTo(items, 0) || zero.anchor != items[0].id {
		t.Fatalf("first Shift+Arrow action should establish an anchor: %#v", zero)
	}
	zero.Clear()
	if !zero.Click(items, 1, true, true) || zero.anchor != items[1].id {
		t.Fatalf("Ctrl-click from empty selection should establish an anchor: %#v", zero)
	}
	zero.Clear()
	if !zero.Click(items, 1, false, true) || !zero.Contains(items[1].id) {
		t.Fatal("Shift-click without an existing anchor should select the clicked item")
	}
	zero.Clear()
	zero.Click(items, 1, false, false)
	zero.RetainAvailable(items[:1])
	if zero.Len() != 0 || zero.anchor != 0 || zero.current != 0 {
		t.Fatalf("refresh must clear vanished selection and navigation IDs: %#v", zero)
	}
	if got := adjacentIndex(nil, 1, 0, true); got != -1 {
		t.Fatalf("adjacent index for empty list = %d, want -1", got)
	}
	if got := clampFirstVisible(9, 0, 0); got != 0 {
		t.Fatalf("empty list scroll offset = %d, want 0", got)
	}
	if got := wheelFirstVisible(2, -120, 10, 3, 0); got != 3 {
		t.Fatalf("wheel step must be at least one row, got %d", got)
	}
	if got := visibleRowsForLayout(100, 20, 0); got != 1 {
		t.Fatalf("invalid row height should still yield one row, got %d", got)
	}
	if got := queryWorkerCount(0, 8); got != 0 {
		t.Fatalf("worker count for no sessions = %d, want 0", got)
	}
	if got := queryWorkerCount(4, 0); got != 0 {
		t.Fatalf("worker count with no worker capacity = %d, want 0", got)
	}
}

func TestLogoAssetHasTransparentBackgroundAndICOVariants(t *testing.T) {
	logo, err := os.Open("logout-icon-clean.png")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(logo)
	logo.Close()
	if err != nil {
		t.Fatalf("decode transparent logo: %v", err)
	}
	if decoded.Bounds().Dx() != 256 || decoded.Bounds().Dy() != 256 {
		t.Fatalf("logo dimensions = %v, want 256x256", decoded.Bounds())
	}
	_, _, _, alpha := decoded.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatalf("logo corner alpha = %d, want transparent background", alpha)
	}

	ico, err := os.ReadFile("app.ico")
	if err != nil {
		t.Fatal(err)
	}
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[0:2]) != 0 || binary.LittleEndian.Uint16(ico[2:4]) != 1 {
		t.Fatal("app.ico has an invalid icon header")
	}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	if count < 4 || len(ico) < 6+count*16 {
		t.Fatalf("app.ico contains %d variants; want at least 4", count)
	}
	sizes := map[int]bool{}
	for i := 0; i < count; i++ {
		w := int(ico[6+i*16])
		if w == 0 {
			w = 256
		}
		sizes[w] = true
	}
	for _, size := range []int{16, 32, 48, 256} {
		if !sizes[size] {
			t.Errorf("app.ico is missing a %dx%d variant", size, size)
		}
	}
}

func sortedIDs(ids []uint32) []uint32 {
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}
