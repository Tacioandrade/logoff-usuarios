//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	gdi32                      = syscall.NewLazyDLL("gdi32.dll")
	wtsapi                     = syscall.NewLazyDLL("wtsapi32.dll")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procSetWindowLongPtrW      = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW        = user32.NewProc("CallWindowProcW")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procSetFocus               = user32.NewProc("SetFocus")
	procGetKeyState            = user32.NewProc("GetKeyState")
	procGetFocus               = user32.NewProc("GetFocus")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procShowWindow             = user32.NewProc("ShowWindow")
	procUpdateWindow           = user32.NewProc("UpdateWindow")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procSendMessageW           = user32.NewProc("SendMessageW")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	procGetWindowTextLengthW   = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
	procInvalidateRect         = user32.NewProc("InvalidateRect")
	procGetClientRect          = user32.NewProc("GetClientRect")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procBeginPaint             = user32.NewProc("BeginPaint")
	procEndPaint               = user32.NewProc("EndPaint")
	procFillRect               = user32.NewProc("FillRect")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procDrawTextW              = user32.NewProc("DrawTextW")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procEllipse                = gdi32.NewProc("Ellipse")
	procMoveToEx               = gdi32.NewProc("MoveToEx")
	procLineTo                 = gdi32.NewProc("LineTo")
	procDrawIconEx             = user32.NewProc("DrawIconEx")
	procGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	procLoadCursorW            = user32.NewProc("LoadCursorW")
	procLoadImageW             = user32.NewProc("LoadImageW")
	procWTSEnumerate           = wtsapi.NewProc("WTSEnumerateSessionsW")
	procWTSQueryInfo           = wtsapi.NewProc("WTSQuerySessionInformationW")
	procWTSFreeMemory          = wtsapi.NewProc("WTSFreeMemory")
	procWTSLogoff              = wtsapi.NewProc("WTSLogoffSession")
	windowProc                 = syscall.NewCallback(wndProc)
	searchWindowProc           = syscall.NewCallback(searchWndProc)
	mainWindow                 uintptr
	appIcon                    uintptr
	sessions                   []session
	selection                  = newSelectionState()
	firstVisible               = 0
	statusMessage              = ""
	pendingMu                  sync.Mutex
	pendingSessions            []session
	pendingLogoffSuccess       bool
	pendingLogoffFailures      int
	pendingLogoffTotal         int
	fontCache                  = map[fontKey]uintptr{}
	allSessions                []session
	searchEdit                 uintptr
	oldSearchWndProc           uintptr
	searchText                 string
)

const (
	wsOverlappedWindow = 0x00CF0000
	wmDestroy          = 0x0002
	wmCreate           = 0x0001
	wmPaint            = 0x000F
	wmLButtonDown      = 0x0201
	wmKeyDown          = 0x0100
	wmChar             = 0x0102
	wmMouseWheel       = 0x020A
	wmSize             = 0x0005
	wmClose            = 0x0010
	wmSetIcon          = 0x0080
	wmSessionsReady    = 0x8001
	wmLogoffDone       = 0x8002
	wmCommand          = 0x0111
	wmSetFont          = 0x0030
	searchControlID    = 2001
	gwlpWndProc        = ^uintptr(3)
	vkControl          = 0x11
	vkShift            = 0x10
	srccopy            = 0x00CC0020
	enSetFocus         = 0x0100
	enKillFocus        = 0x0200
	esAutoHScroll      = 0x0080
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsBorder           = 0x00800000
	wsTabStop          = 0x00010000
	enChange           = 0x0300
	emSetCueBanner     = 0x1501
	swpNoZOrder        = 0x0004
	swpNoActivate      = 0x0010
	wtsUserName        = 5
	wtsDomainName      = 7
	wtsActive          = 0
	mbOKCancel         = 0x00000001
	mbIconQuestion     = 0x00000020
	iconSmall          = 0
	iconBig            = 1
	dtLeft             = 0x0000
	dtVCenter          = 0x0004
	dtSingleLine       = 0x0020
	dtNoPrefix         = 0x0800
	transparent        = 1
	psSolid            = 0
	fwRegular          = 400
	fwSemiBold         = 600
	fwBold             = 700
	defaultCharset     = 1
	cleartypeQuality   = 5
	searchFieldY       = 130
	searchFieldHeight  = 34
	listPanelTop       = 174
	rowTop             = 182
	rowHeight          = 56
	buttonHeight       = 46
	maxSessionWorkers  = 8
)

type point struct{ X, Y int32 }
type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}
type rect struct{ Left, Top, Right, Bottom int32 }
type paintStruct struct {
	HDC       uintptr
	Erase     int32
	Paint     rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}
type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSmall  uintptr
}
type wtsSessionInfo struct {
	SessionID      uint32
	WinStationName *uint16
	State          uint32
}
type fontKey struct {
	weight int
	size   int
}

func main() {
	// A fila de mensagens Win32 deve permanecer na thread que criou a janela.
	runtime.LockOSThread()
	instance, _, _ := procGetModuleHandleW.Call(0)
	arrowCursor, _, _ := procLoadCursorW.Call(0, 32512)                     // IDC_ARROW padrão do Windows.
	appIcon, _, _ = procLoadImageW.Call(instance, 1, 1, 32, 32, 0x00000040) // Ícone embutido no executável.
	className, _ := syscall.UTF16PtrFromString("LogoffUsersWindow")
	title, _ := syscall.UTF16PtrFromString("Encerrar sessão de usuário")
	background := uintptr(6) // COLOR_WINDOW + 1
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: windowProc, Instance: instance, Icon: appIcon, Cursor: arrowCursor, IconSmall: appIcon, Background: background, ClassName: className}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		fatal("Não foi possível iniciar a janela.")
		return
	}
	mainWindow, _, _ = procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), wsOverlappedWindow,
		0x80000000, 0x80000000, 560, 530, 0, 0, instance, 0)
	if mainWindow == 0 {
		fatal("Não foi possível criar a janela.")
		return
	}
	if appIcon != 0 {
		procSendMessageW.Call(mainWindow, wmSetIcon, iconBig, appIcon)
		procSendMessageW.Call(mainWindow, wmSetIcon, iconSmall, appIcon)
	}
	procShowWindow.Call(mainWindow, 1)
	procUpdateWindow.Call(mainWindow)
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmCreate:
		mainWindow = hwnd
		createSearchField(hwnd)
		statusMessage = "Carregando sessões..."
		invalidate(hwnd)
		loadSessionsAsync(hwnd)
		return 0
	case wmSessionsReady:
		applyPendingSessions(hwnd)
		return 0
	case wmLogoffDone:
		pendingMu.Lock()
		succeeded := pendingLogoffSuccess
		failed, total := pendingLogoffFailures, pendingLogoffTotal
		pendingMu.Unlock()
		if succeeded {
			statusMessage = "Logoff concluído. Atualizando a lista..."
		} else {
			statusMessage = fmt.Sprintf("%d de %d sessões não puderam ser encerradas. Atualizando a lista...", failed, total)
		}
		loadSessionsAsyncWithStatus(hwnd, statusMessage)
		return 0
	case wmCommand:
		if uint16(wParam&0xffff) == searchControlID {
			notify := uint16(wParam >> 16)
			if notify == enChange {
				searchText = readSearchText()
				applySearchFilter(hwnd)
				return 0
			}
			if notify == enSetFocus || notify == enKillFocus {
				invalidate(hwnd)
			}
		}
	case wmPaint:
		paintWindow(hwnd)
		return 0
	case wmLButtonDown:
		handleClick(hwnd, int(int16(uint16(lParam))), int(int16(uint16(lParam>>16))))
		return 0
	case wmKeyDown:
		action := actionForKey(uint16(wParam), keyDown(vkControl), keyDown(vkShift))
		switch action {
		case keyActionClearSelection:
			clearSelection()
			statusMessage = "Seleção desfeita."
			invalidate(hwnd)
			return 0
		case keyActionSelectAll:
			selectAllFiltered(hwnd)
			return 0
		case keyActionFocusSearch:
			focusSearch()
			return 0
		case keyActionExtendUp, keyActionExtendDown:
			selectAdjacent(hwnd, action == keyActionExtendDown)
			return 0
		}
	case wmMouseWheel:
		var client rect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		visible := visibleRowCount(int(client.Bottom) - 112)
		firstVisible = wheelFirstVisible(firstVisible, int(int16(uint16(wParam>>16))), len(sessions), visible, 3)
		clampScroll(hwnd)
		invalidate(hwnd)
		return 0
	case wmSize:
		if searchEdit != 0 {
			width := int(uint16(lParam))
			procSetWindowPos.Call(searchEdit, 0, 34, searchFieldY+6, uintptr(max(100, width-68)), searchFieldHeight-8, swpNoZOrder|swpNoActivate)
		}
		clampScroll(hwnd)
		invalidate(hwnd)
		return 0
	case wmClose:
		procPostQuitMessage.Call(0)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func paintWindow(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	var client rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	width, height := int(client.Right), int(client.Bottom)
	bufferDC, _, _ := procCreateCompatibleDC.Call(hdc)
	bufferBitmap, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(max(1, width)), uintptr(max(1, height)))
	oldBitmap, _, _ := procSelectObject.Call(bufferDC, bufferBitmap)
	drawDC := bufferDC
	procSetBkMode.Call(hdc, transparent)
	procSetBkMode.Call(drawDC, transparent)
	drawRect(drawDC, 0, 0, width, height, rgb(246, 248, 252))

	if appIcon != 0 {
		procDrawIconEx.Call(drawDC, 26, 23, appIcon, 34, 34, 0, 0, 3)
	}
	drawText(drawDC, "Encerrar sessão de usuário", 72, 23, width-94, 29, rgb(26, 36, 52), fwBold, 19)
	drawText(drawDC, "Gerencie as sessões abertas neste computador", 72, 53, width-94, 22, rgb(103, 116, 137), fwRegular, 12)
	drawText(drawDC, "Sessões abertas", 26, 96, width-52, 24, rgb(39, 51, 70), fwSemiBold, 14)
	visibleRows := visibleRowCount(height - 112)
	rangeLabel := "0 sessões"
	if len(sessions) > 0 {
		first := firstVisible + 1
		last := min(firstVisible+visibleRows, len(sessions))
		rangeLabel = fmt.Sprintf("Mostrando %d–%d de %d", first, last, len(sessions))
	}
	drawTextRight(drawDC, rangeLabel, 26, 97, width-52, 22, rgb(115, 129, 150), fwRegular, 11)
	searchBorder := rgb(222, 228, 237)
	if focused, _, _ := procGetFocus.Call(); focused == searchEdit {
		searchBorder = rgb(37, 99, 235)
	}
	roundRect(drawDC, 24, searchFieldY, width-24, searchFieldY+searchFieldHeight, 8, rgb(255, 255, 255), searchBorder)

	listBottom := height - 112
	if listBottom < rowTop+rowHeight {
		listBottom = rowTop + rowHeight
	}
	roundRect(drawDC, 20, listPanelTop, width-20, listBottom, 12, rgb(255, 255, 255), rgb(225, 231, 240))
	visibleRows = visibleRowCount(listBottom)
	needsScroll := len(sessions) > visibleRows
	rowRight := width - 27
	if needsScroll {
		rowRight = width - 43
	}
	for slot := 0; slot < visibleRows; slot++ {
		index := firstVisible + slot
		if index >= len(sessions) {
			break
		}
		s := sessions[index]
		y := rowTop + slot*rowHeight
		if isSelected(s.id) {
			roundRect(drawDC, 27, y+3, rowRight, y+rowHeight-3, 9, activeSelectionColor(), activeSelectionColor())
		}
		avatarFill, avatarText := rgb(231, 239, 251), rgb(48, 104, 190)
		if isSelected(s.id) {
			avatarFill, avatarText = activeSelectionColor(), rgb(255, 255, 255)
		}
		drawCircle(drawDC, 38, y+13, 30, avatarFill)
		initial := "?"
		if len(s.name) > 0 {
			initial = string([]rune(s.name)[0])
		}
		drawTextCentered(drawDC, initial, 38, y+13, 30, 30, avatarText, fwSemiBold, 13)
		nameColor, detailColor := rgb(39, 51, 70), rgb(115, 129, 150)
		if isSelected(s.id) {
			nameColor, detailColor = rgb(255, 255, 255), rgb(224, 235, 255)
		}
		drawText(drawDC, s.name, 80, y+7, rowRight-92, 21, nameColor, fwSemiBold, 13)
		drawText(drawDC, fmt.Sprintf("%s  ·  Sessão %d", s.state, s.id), 80, y+29, rowRight-92, 18, detailColor, fwRegular, 11)
	}
	if needsScroll {
		drawSessionScrollbar(drawDC, width, rowTop, listBottom, visibleRows)
	}
	if len(sessions) == 0 {
		drawTextCentered(drawDC, "Nenhuma sessão de usuário encontrada", 34, 160, width-68, 44, rgb(115, 129, 150), fwRegular, 12)
	}

	statusY := height - 96
	status := statusMessage
	if status == "" {
		status = "Selecione uma sessão para continuar."
	}
	drawText(drawDC, status, 26, statusY, width-52, 20, rgb(103, 116, 137), fwRegular, 11)

	buttonY := height - 66
	buttonColor, buttonTextColor := rgb(211, 220, 233), rgb(128, 142, 161)
	if selection.Len() > 0 {
		buttonColor, buttonTextColor = activeSelectionColor(), rgb(255, 255, 255)
	}
	roundRect(drawDC, 22, buttonY, width-22, buttonY+buttonHeight, 10, buttonColor, buttonColor)
	buttonLabel := "Encerrar sessão selecionada"
	if selection.Len() > 1 {
		buttonLabel = fmt.Sprintf("Encerrar %d sessões selecionadas", selection.Len())
	}
	drawTextCentered(drawDC, buttonLabel, 30, buttonY, width-60, buttonHeight, buttonTextColor, fwSemiBold, 13)
	procBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), bufferDC, 0, 0, srccopy)
	procSelectObject.Call(bufferDC, oldBitmap)
	procDeleteObject.Call(bufferBitmap)
	procDeleteDC.Call(bufferDC)
	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

func handleClick(hwnd uintptr, x, y int) {
	var client rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	height := int(client.Bottom)
	width := int(client.Right)
	listBottom := height - 112
	visibleRows := visibleRowCount(listBottom)
	if needsScrollbar(len(sessions), visibleRows) && x >= width-42 && x <= width-18 && y >= rowTop+4 && y < listBottom-8 {
		trackTop, trackBottom := rowTop+4, listBottom-8
		maxFirst := len(sessions) - visibleRows
		firstVisible = (y - trackTop) * maxFirst / max(1, trackBottom-trackTop)
		clampScroll(hwnd)
		invalidate(hwnd)
		return
	}
	if x >= 20 && x <= width-20 && y >= rowTop && y < listBottom && (!needsScrollbar(len(sessions), visibleRows) || x < width-42) {
		slot := (y - rowTop) / rowHeight
		index := firstVisible + slot
		if slot >= 0 && slot < visibleRows && index < len(sessions) {
			procSetFocus.Call(hwnd)
			selection.Click(sessions, index, keyDown(vkControl), keyDown(vkShift))
			statusMessage = "Ctrl+clique/Shift+clique: vários · Ctrl+A: todos filtrados."
			invalidate(hwnd)
		}
		return
	}
	buttonY := height - 66
	if x >= 22 && x <= width-22 && y >= buttonY && y <= buttonY+buttonHeight {
		logoffSelected(hwnd)
	}
}

func visibleRowCount(listBottom int) int {
	return visibleRowsForLayout(listBottom, rowTop, rowHeight)
}

func needsScrollbar(total, visible int) bool { return total > visible }

func drawSessionScrollbar(hdc uintptr, width, top, bottom, visible int) {
	trackTop, trackBottom := top+4, bottom-8
	trackHeight := trackBottom - trackTop
	if trackHeight <= 0 {
		return
	}
	roundRect(hdc, width-30, trackTop, width-25, trackBottom, 3, rgb(239, 243, 248), rgb(239, 243, 248))
	thumbHeight := trackHeight * visible / len(sessions)
	if thumbHeight < 26 {
		thumbHeight = 26
	}
	if thumbHeight > trackHeight {
		thumbHeight = trackHeight
	}
	maxFirst := len(sessions) - visible
	thumbTop := trackTop
	if maxFirst > 0 {
		thumbTop += (trackHeight - thumbHeight) * firstVisible / maxFirst
	}
	roundRect(hdc, width-32, thumbTop, width-23, thumbTop+thumbHeight, 4, rgb(160, 176, 199), rgb(160, 176, 199))
}

func loadSessionsAsync(hwnd uintptr) {
	loadSessionsAsyncWithStatus(hwnd, "Carregando sessões...")
}

func loadSessionsAsyncWithStatus(hwnd uintptr, status string) {
	statusMessage = status
	invalidate(hwnd)
	go func() {
		found := enumerateSessions()
		pendingMu.Lock()
		pendingSessions = found
		pendingMu.Unlock()
		procPostMessageW.Call(hwnd, wmSessionsReady, 0, 0)
	}()
}

func createSearchField(hwnd uintptr) {
	className, _ := syscall.UTF16PtrFromString("EDIT")
	empty, _ := syscall.UTF16PtrFromString("")
	var client rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	searchEdit, _, _ = procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(empty)),
		wsChild|wsVisible|wsTabStop|esAutoHScroll, 34, searchFieldY+6, uintptr(int(client.Right)-68), searchFieldHeight-8,
		hwnd, searchControlID, 0, 0)
	if searchEdit == 0 {
		return
	}
	procSendMessageW.Call(searchEdit, wmSetFont, cachedFont(fwRegular, 13), 1)
	oldSearchWndProc, _, _ = procSetWindowLongPtrW.Call(searchEdit, gwlpWndProc, searchWindowProc)
	hint, _ := syscall.UTF16PtrFromString("Pesquisar por nome de usuário...")
	procSendMessageW.Call(searchEdit, emSetCueBanner, 1, uintptr(unsafe.Pointer(hint)))
}

func readSearchText() string {
	length, _, _ := procGetWindowTextLengthW.Call(searchEdit)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	procGetWindowTextW.Call(searchEdit, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer)
}

func applySearchFilter(hwnd uintptr) {
	sessions = FilterSessions(allSessions, searchText)
	needle := strings.TrimSpace(searchText)
	firstVisible = 0
	if needle != "" && len(sessions) == 0 {
		statusMessage = "Nenhum usuário corresponde à pesquisa."
	} else if needle != "" {
		statusMessage = fmt.Sprintf("%d resultado(s) · Ctrl/Shift+clique ou Ctrl+A seleciona vários.", len(sessions))
	} else if len(sessions) == 0 {
		statusMessage = "Nenhuma sessão aberta foi encontrada."
	} else {
		statusMessage = "Ctrl+clique/Shift+clique: vários · Ctrl+A: todos filtrados."
	}
	invalidate(hwnd)
}

func searchWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if message == wmKeyDown {
		switch actionForKey(uint16(wParam), keyDown(vkControl), keyDown(vkShift)) {
		case keyActionClearSelection:
			clearSelection()
			statusMessage = "Seleção desfeita."
			invalidate(mainWindow)
			return 0
		case keyActionFocusSearch:
			focusSearch()
			return 0
		case keyActionSelectAll:
			selectAllFiltered(mainWindow)
			return 0
		}
	}
	if message == wmChar && wParam == 1 && keyDown(vkControl) {
		return 0
	}
	if oldSearchWndProc != 0 {
		result, _, _ := procCallWindowProcW.Call(oldSearchWndProc, hwnd, uintptr(message), wParam, lParam)
		return result
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func selectAllFiltered(hwnd uintptr) {
	selection.SelectVisible(sessions)
	if selection.Len() == 0 {
		statusMessage = "Não há usuários visíveis para selecionar."
	} else {
		statusMessage = fmt.Sprintf("%d usuário(s) visível(is) selecionado(s).", selection.Len())
	}
	invalidate(hwnd)
}

func isSelected(sessionID uint32) bool { return selection.Contains(sessionID) }

func clearSelection() { selection.Clear() }

func focusSearch() {
	if searchEdit == 0 {
		return
	}
	procSetFocus.Call(searchEdit)
	procSendMessageW.Call(searchEdit, 0x00B1, 0, ^uintptr(0)) // EM_SETSEL seleciona o texto atual.
}

func selectAdjacent(hwnd uintptr, down bool) {
	if len(sessions) == 0 {
		return
	}
	index := adjacentIndex(sessions, selection.current, firstVisible, down)
	selection.ExtendTo(sessions, index)
	var client rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	visible := visibleRowCount(int(client.Bottom) - 112)
	if index < firstVisible {
		firstVisible = index
	} else if index >= firstVisible+visible {
		firstVisible = index - visible + 1
	}
	clampScroll(hwnd)
	invalidate(hwnd)
}

func keyDown(key uintptr) bool {
	state, _, _ := procGetKeyState.Call(key)
	return uint16(state)&0x8000 != 0
}

func applyPendingSessions(hwnd uintptr) {
	pendingMu.Lock()
	allSessions = append([]session(nil), pendingSessions...)
	pendingMu.Unlock()
	selection.RetainAvailable(allSessions)
	applySearchFilter(hwnd)
}

func enumerateSessions() []session {
	var base uintptr
	var count uint32
	r, _, _ := procWTSEnumerate.Call(0, 0, 1, uintptr(unsafe.Pointer(&base)), uintptr(unsafe.Pointer(&count)))
	if r == 0 {
		return nil
	}
	defer procWTSFreeMemory.Call(base)
	items := unsafe.Slice((*wtsSessionInfo)(unsafe.Pointer(base)), count)
	out := make([]session, len(items))
	for i, item := range items {
		state := "Desconectada"
		if item.State == wtsActive {
			state = "Ativa"
		}
		out[i] = session{id: item.SessionID, state: state}
	}
	workers := queryWorkerCount(len(out), maxSessionWorkers)
	if workers == 0 {
		return nil
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				user := queryString(out[i].id, wtsUserName)
				if user == "" {
					continue
				}
				domain := queryString(out[i].id, wtsDomainName)
				if domain != "" {
					user = domain + `\` + user
				}
				out[i].name = user
			}
		}()
	}
	for i := range out {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return namedSessions(out)
}

func queryString(id, info uint32) string {
	var buffer uintptr
	var bytes uint32
	r, _, _ := procWTSQueryInfo.Call(0, uintptr(id), uintptr(info), uintptr(unsafe.Pointer(&buffer)), uintptr(unsafe.Pointer(&bytes)))
	if r == 0 || buffer == 0 {
		return ""
	}
	defer procWTSFreeMemory.Call(buffer)
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(buffer)), int(bytes/2)))
}

func logoffSelected(hwnd uintptr) {
	chosen := selectedSessions()
	if len(chosen) == 0 {
		return
	}
	messageText := fmt.Sprintf("Deseja encerrar %d sessões selecionadas? Os programas serão fechados e alterações não salvas podem ser perdidas.", len(chosen))
	if len(chosen) == 1 {
		messageText = fmt.Sprintf("Deseja encerrar a sessão de %s? Os programas abertos serão fechados e alterações não salvas podem ser perdidas.", chosen[0].name)
	}
	message, _ := syscall.UTF16PtrFromString(messageText)
	title, _ := syscall.UTF16PtrFromString("Confirmar encerramento")
	r, _, _ := procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), mbOKCancel|mbIconQuestion)
	if r != 1 {
		return
	}
	idList := make([]uint32, len(chosen))
	for i, s := range chosen {
		idList[i] = s.id
	}
	clearSelection()
	statusMessage = fmt.Sprintf("Encerrando %d sessão(ões)...", len(idList))
	invalidate(hwnd)
	go func(ids []uint32) {
		failures := 0
		for _, sessionID := range ids {
			result, _, _ := procWTSLogoff.Call(0, uintptr(sessionID), 0)
			if result == 0 {
				failures++
			}
		}
		pendingMu.Lock()
		pendingLogoffSuccess = failures == 0
		pendingLogoffFailures = failures
		pendingLogoffTotal = len(ids)
		pendingMu.Unlock()
		procPostMessageW.Call(hwnd, wmLogoffDone, 0, 0)
	}(idList)
}

func selectedSessions() []session {
	return selection.Sessions(allSessions)
}

func clampScroll(hwnd uintptr) {
	var client rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	visible := (int(client.Bottom) - 112 - rowTop - 4) / rowHeight
	if visible < 1 {
		visible = 1
	}
	firstVisible = clampFirstVisible(firstVisible, len(sessions), visible)
}

func drawText(hdc uintptr, value string, x, y, width, height int, color uint32, weight, size int) {
	drawTextAligned(hdc, value, rect{Left: int32(x), Top: int32(y), Right: int32(x + width), Bottom: int32(y + height)}, color, weight, size, dtLeft|dtVCenter|dtSingleLine|dtNoPrefix)
}
func drawTextRight(hdc uintptr, value string, x, y, width, height int, color uint32, weight, size int) {
	drawTextAligned(hdc, value, rect{Left: int32(x), Top: int32(y), Right: int32(x + width), Bottom: int32(y + height)}, color, weight, size, 0x0002|dtVCenter|dtSingleLine|dtNoPrefix)
}
func drawTextCentered(hdc uintptr, value string, x, y, width, height int, color uint32, weight, size int) {
	drawTextAligned(hdc, value, rect{Left: int32(x), Top: int32(y), Right: int32(x + width), Bottom: int32(y + height)}, color, weight, size, 0x0001|dtVCenter|dtSingleLine|dtNoPrefix)
}
func drawTextAligned(hdc uintptr, value string, bounds rect, color uint32, weight, size int, flags uintptr) {
	font := cachedFont(weight, size)
	oldFont, _, _ := procSelectObject.Call(hdc, font)
	procSetTextColor.Call(hdc, uintptr(color))
	text, _ := syscall.UTF16PtrFromString(value)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), flags)
	procSelectObject.Call(hdc, oldFont)
}

func cachedFont(weight, size int) uintptr {
	key := fontKey{weight: weight, size: size}
	font := fontCache[key]
	if font == 0 {
		fontName, _ := syscall.UTF16PtrFromString("Segoe UI")
		font, _, _ = procCreateFontW.Call(uintptr(-size), 0, 0, 0, uintptr(weight), 0, 0, 0, defaultCharset, 0, 0, cleartypeQuality, 0, uintptr(unsafe.Pointer(fontName)))
		fontCache[key] = font
	}
	return font
}
func drawRect(hdc uintptr, left, top, right, bottom int, color uint32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(color))
	r := rect{Left: int32(left), Top: int32(top), Right: int32(right), Bottom: int32(bottom)}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
	procDeleteObject.Call(brush)
}
func roundRect(hdc uintptr, left, top, right, bottom, radius int, fill, border uint32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(fill))
	pen, _, _ := procCreatePen.Call(psSolid, 1, uintptr(border))
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procRoundRect.Call(hdc, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom), uintptr(radius), uintptr(radius))
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
}
func drawCircle(hdc uintptr, left, top, size int, fill uint32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(fill))
	pen, _, _ := procCreatePen.Call(psSolid, 1, uintptr(fill))
	oldBrush, _, _ := procSelectObject.Call(hdc, brush)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procEllipse.Call(hdc, uintptr(left), uintptr(top), uintptr(left+size), uintptr(top+size))
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(brush)
	procDeleteObject.Call(pen)
}
func rgb(r, g, b uint32) uint32 { return r | g<<8 | b<<16 }
func invalidate(hwnd uintptr)   { procInvalidateRect.Call(hwnd, 0, 0) }
func fatal(message string) {
	msg, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("Logoff de usuários")
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), 0)
}
