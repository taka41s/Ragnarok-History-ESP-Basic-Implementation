package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// ============================================================================
// OFFSETS - Ragnarok Online
// ============================================================================
const (
	SINGLETON_BASE     = 0x119FE28
	LOCAL_PLAYER_GID   = 0x158356C
	LOCAL_PLAYER_AID   = 0x1583570
	LOCAL_MAP_NAME     = 0x1583574
	LOCAL_HP           = 0x15874D0
	LOCAL_SP           = 0x15874D4
	LOCAL_MAXHP        = 0x15874D8
	LOCAL_MAXSP        = 0x15874DC

	OFFSET_MANAGER_PTR = 0x04
	OFFSET_INIT_FLAG   = 0x58
	OFFSET_ACTORLIST   = 0xCC
	OFFSET_LOCAL_ACTOR = 0x2C
	OFFSET_LIST_HEAD   = 0x10

	ACTOR_GID      = 0x110
	ACTOR_WORLD_X  = 0x10
	ACTOR_WORLD_Y  = 0x18
	ACTOR_TYPE     = 0x70
	ACTOR_SCREEN_X = 0x0AC
	ACTOR_SCREEN_Y = 0x0B0
)

// Actor Types
const (
	TYPE_PLAYER = 0
	TYPE_NPC    = 1
	TYPE_ITEM   = 2
	TYPE_MOB    = 5
	TYPE_PET    = 7
)

// ============================================================================
// WINDOWS API
// ============================================================================
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procRegisterClassExW         = user32.NewProc("RegisterClassExW")
	procCreateWindowExW          = user32.NewProc("CreateWindowExW")
	procDefWindowProcW           = user32.NewProc("DefWindowProcW")
	procPeekMessageW             = user32.NewProc("PeekMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procShowWindow               = user32.NewProc("ShowWindow")
	procUpdateLayeredWindow      = user32.NewProc("UpdateLayeredWindow")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetClientRect            = user32.NewProc("GetClientRect")
	procClientToScreen           = user32.NewProc("ClientToScreen")
	procGetDC                    = user32.NewProc("GetDC")
	procReleaseDC                = user32.NewProc("ReleaseDC")
	procGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")

	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = kernel32.NewProc("Process32FirstW")
	procProcess32Next            = kernel32.NewProc("Process32NextW")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
)

const (
	PROCESS_ALL_ACCESS = 0x1F0FFF
	TH32CS_SNAPPROCESS = 0x00000002
	MAX_PATH           = 260
	WS_EX_LAYERED      = 0x00080000
	WS_EX_TRANSPARENT  = 0x00000020
	WS_EX_TOPMOST      = 0x00000008
	WS_EX_TOOLWINDOW   = 0x00000080
	WS_EX_NOACTIVATE   = 0x08000000
	WS_POPUP           = 0x80000000
	SW_SHOW            = 5
	CS_HREDRAW         = 0x0002
	CS_VREDRAW         = 0x0001
	ULW_ALPHA          = 0x00000002
	AC_SRC_OVER        = 0x00
	AC_SRC_ALPHA       = 0x01
	WM_DESTROY         = 0x0002
	PM_REMOVE          = 0x0001
	DIB_RGB_COLORS     = 0
	BI_RGB             = 0

	VK_F1  = 0x70
	VK_F2  = 0x71
	VK_F3  = 0x72
	VK_F4  = 0x73
	VK_END = 0x23
)

// ============================================================================
// STRUCTS
// ============================================================================
type WNDCLASSEXW struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background syscall.Handle
	MenuName, ClassName                *uint16
	IconSm                             syscall.Handle
}

type MSG struct {
	Hwnd           syscall.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             struct{ X, Y int32 }
}

type POINT struct{ X, Y int32 }
type SIZE struct{ CX, CY int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }

type BLENDFUNCTION struct {
	BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte
}

type BITMAPINFOHEADER struct {
	Size          uint32
	Width, Height int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors [1]uint32
}

type PROCESSENTRY32W struct {
	Size, Usage, ProcessID             uint32
	DefaultHeapID                      uintptr
	ModuleID, Threads, ParentProcessID uint32
	PriorityClassBase                  int32
	Flags                              uint32
	ExeFile                            [MAX_PATH]uint16
}

type Actor struct {
	GID              uint32
	Type             uint32
	WorldX, WorldY   float32
	ScreenX, ScreenY int32
	Distance         float32
}

// ============================================================================
// GLOBALS
// ============================================================================
var (
	roMem       *Memory
	gameHwnd    syscall.Handle
	overlayHwnd syscall.Handle
	width       int32 = 1024
	height      int32 = 768
	offsetX     int32
	offsetY     int32
	pixels      unsafe.Pointer
	memDC       uintptr
	memBitmap   uintptr

	showPlayers = true
	showMobs    = true
	showItems   = true
	showLines   = true
	running     = true
)

// ============================================================================
// MEMORY
// ============================================================================
type Memory struct {
	handle syscall.Handle
	pid    uint32
}

func NewMemory(processName string) (*Memory, error) {
	pid, err := findProcess(processName)
	if err != nil {
		return nil, err
	}
	handle, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if handle == 0 {
		return nil, fmt.Errorf("open failed")
	}
	return &Memory{handle: syscall.Handle(handle), pid: pid}, nil
}

func (m *Memory) Close() { procCloseHandle.Call(uintptr(m.handle)) }

func (m *Memory) ReadUint32(addr uintptr) uint32 {
	var val uint32
	procReadProcessMemory.Call(uintptr(m.handle), addr, uintptr(unsafe.Pointer(&val)), 4, 0)
	return val
}

func (m *Memory) ReadInt32(addr uintptr) int32 {
	return int32(m.ReadUint32(addr))
}

func (m *Memory) ReadFloat32(addr uintptr) float32 {
	bits := m.ReadUint32(addr)
	return math.Float32frombits(bits)
}

func (m *Memory) ReadString(addr uintptr, maxLen int) string {
	buf := make([]byte, maxLen)
	procReadProcessMemory.Call(uintptr(m.handle), addr, uintptr(unsafe.Pointer(&buf[0])), uintptr(maxLen), 0)
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func findProcess(name string) (uint32, error) {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return 0, fmt.Errorf("snapshot failed")
	}
	defer procCloseHandle.Call(snapshot)
	var entry PROCESSENTRY32W
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return 0, fmt.Errorf("enum failed")
	}
	for {
		if strings.EqualFold(syscall.UTF16ToString(entry.ExeFile[:]), name) {
			return entry.ProcessID, nil
		}
		ret, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}
	return 0, fmt.Errorf("not found")
}

// ============================================================================
// GAME FUNCTIONS
// ============================================================================
func getManager() uintptr {
	if roMem.ReadUint32(SINGLETON_BASE+OFFSET_INIT_FLAG) == 1 {
		return uintptr(roMem.ReadUint32(SINGLETON_BASE + OFFSET_MANAGER_PTR))
	}
	return 0
}

func getLocalActor() uintptr {
	m := getManager()
	if m == 0 {
		return 0
	}
	al := uintptr(roMem.ReadUint32(m + OFFSET_ACTORLIST))
	return uintptr(roMem.ReadUint32(al + OFFSET_LOCAL_ACTOR))
}

func getPlayerScreenPos() (int32, int32) {
	a := getLocalActor()
	if a == 0 {
		return 0, 0
	}
	return roMem.ReadInt32(a + ACTOR_SCREEN_X), roMem.ReadInt32(a + ACTOR_SCREEN_Y)
}

func getPlayerWorldPos() (float32, float32) {
	a := getLocalActor()
	if a == 0 {
		return 0, 0
	}
	return roMem.ReadFloat32(a + ACTOR_WORLD_X), roMem.ReadFloat32(a + ACTOR_WORLD_Y)
}

func getPlayerGID() uint32 {
	return roMem.ReadUint32(LOCAL_PLAYER_GID)
}

func getPlayerHP() (int32, int32) {
	return roMem.ReadInt32(LOCAL_HP), roMem.ReadInt32(LOCAL_MAXHP)
}

func getPlayerSP() (int32, int32) {
	return roMem.ReadInt32(LOCAL_SP), roMem.ReadInt32(LOCAL_MAXSP)
}

func getMapName() string {
	return roMem.ReadString(LOCAL_MAP_NAME, 24)
}

func getAllActors() []Actor {
	m := getManager()
	if m == 0 {
		return nil
	}
	al := uintptr(roMem.ReadUint32(m + OFFSET_ACTORLIST))
	if al == 0 {
		return nil
	}
	headPtr := roMem.ReadUint32(al + OFFSET_LIST_HEAD)
	if headPtr == 0 {
		return nil
	}
	curPtr := roMem.ReadUint32(uintptr(headPtr))

	pwx, pwy := getPlayerWorldPos()
	localGID := getPlayerGID()

	var actors []Actor
	visited := make(map[uint32]bool)

	for i := 0; i < 500; i++ {
		if curPtr == headPtr || curPtr == 0 || visited[curPtr] {
			break
		}
		visited[curPtr] = true

		actorPtr := uintptr(roMem.ReadUint32(uintptr(curPtr) + 8))
		if actorPtr != 0 {
			gid := roMem.ReadUint32(actorPtr + ACTOR_GID)
			if gid != 0 && gid != localGID {
				sx := roMem.ReadInt32(actorPtr + ACTOR_SCREEN_X)
				sy := roMem.ReadInt32(actorPtr + ACTOR_SCREEN_Y)
				wx := roMem.ReadFloat32(actorPtr + ACTOR_WORLD_X)
				wy := roMem.ReadFloat32(actorPtr + ACTOR_WORLD_Y)

				dx := wx - pwx
				dy := wy - pwy
				dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

				actors = append(actors, Actor{
					GID:      gid,
					Type:     roMem.ReadUint32(actorPtr + ACTOR_TYPE),
					WorldX:   wx,
					WorldY:   wy,
					ScreenX:  sx,
					ScreenY:  sy,
					Distance: dist,
				})
			}
		}
		curPtr = roMem.ReadUint32(uintptr(curPtr))
	}
	return actors
}

// ============================================================================
// WINDOW
// ============================================================================
var foundHwnd syscall.Handle
var targetPid uint32

func enumCallback(hwnd uintptr, lParam uintptr) uintptr {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == targetPid {
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible != 0 {
			foundHwnd = syscall.Handle(hwnd)
			return 0
		}
	}
	return 1
}

func findGameWindow(pid uint32) syscall.Handle {
	foundHwnd = 0
	targetPid = pid
	procEnumWindows.Call(syscall.NewCallback(enumCallback), 0)
	return foundHwnd
}

// ============================================================================
// PIXEL DRAWING
// ============================================================================
func setPixel(x, y int32, r, g, b, a uint8) {
	if x < 0 || x >= width || y < 0 || y >= height || pixels == nil {
		return
	}
	idx := ((height - 1 - y) * width + x) * 4
	p := (*[4]byte)(unsafe.Pointer(uintptr(pixels) + uintptr(idx)))
	p[0] = uint8(uint16(b) * uint16(a) / 255)
	p[1] = uint8(uint16(g) * uint16(a) / 255)
	p[2] = uint8(uint16(r) * uint16(a) / 255)
	p[3] = a
}

func clearPixels() {
	if pixels == nil {
		return
	}
	size := width * height * 4
	for i := int32(0); i < size; i++ {
		*(*byte)(unsafe.Pointer(uintptr(pixels) + uintptr(i))) = 0
	}
}

func drawLine(x1, y1, x2, y2 int32, r, g, b, a uint8, thick int32) {
	dx := int32(math.Abs(float64(x2 - x1)))
	dy := int32(math.Abs(float64(y2 - y1)))
	steps := dx
	if dy > dx {
		steps = dy
	}
	if steps == 0 {
		steps = 1
	}
	xInc := float64(x2-x1) / float64(steps)
	yInc := float64(y2-y1) / float64(steps)
	x := float64(x1)
	y := float64(y1)
	half := thick / 2
	for i := int32(0); i <= steps; i++ {
		for t := -half; t <= half; t++ {
			setPixel(int32(x)+t, int32(y), r, g, b, a)
			setPixel(int32(x), int32(y)+t, r, g, b, a)
		}
		x += xInc
		y += yInc
	}
}

func drawBox(cx, cy, size int32, r, g, b, a uint8, thick int32) {
	drawLine(cx-size, cy-size, cx+size, cy-size, r, g, b, a, thick)
	drawLine(cx+size, cy-size, cx+size, cy+size, r, g, b, a, thick)
	drawLine(cx+size, cy+size, cx-size, cy+size, r, g, b, a, thick)
	drawLine(cx-size, cy+size, cx-size, cy-size, r, g, b, a, thick)
}

func drawCross(cx, cy, size int32, r, g, b, a uint8, thick int32) {
	drawLine(cx-size, cy, cx+size, cy, r, g, b, a, thick)
	drawLine(cx, cy-size, cx, cy+size, r, g, b, a, thick)
}

func drawFilledRect(x, y, w, h int32, r, g, b, a uint8) {
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			setPixel(px, py, r, g, b, a)
		}
	}
}

// Simple digit patterns (3x5)
var digits = map[rune][5]uint8{
	'0': {0x7, 0x5, 0x5, 0x5, 0x7},
	'1': {0x2, 0x6, 0x2, 0x2, 0x7},
	'2': {0x7, 0x1, 0x7, 0x4, 0x7},
	'3': {0x7, 0x1, 0x7, 0x1, 0x7},
	'4': {0x5, 0x5, 0x7, 0x1, 0x1},
	'5': {0x7, 0x4, 0x7, 0x1, 0x7},
	'6': {0x7, 0x4, 0x7, 0x5, 0x7},
	'7': {0x7, 0x1, 0x1, 0x1, 0x1},
	'8': {0x7, 0x5, 0x7, 0x5, 0x7},
	'9': {0x7, 0x5, 0x7, 0x1, 0x7},
	'.': {0x0, 0x0, 0x0, 0x0, 0x2},
	'-': {0x0, 0x0, 0x7, 0x0, 0x0},
	' ': {0x0, 0x0, 0x0, 0x0, 0x0},
}

func drawDigit(x, y int32, c rune, r, g, b, a uint8, scale int32) int32 {
	pattern, ok := digits[c]
	if !ok {
		return 4 * scale
	}
	for row := int32(0); row < 5; row++ {
		bits := pattern[row]
		for col := int32(0); col < 3; col++ {
			if bits&(1<<(2-col)) != 0 {
				for sy := int32(0); sy < scale; sy++ {
					for sx := int32(0); sx < scale; sx++ {
						setPixel(x+col*scale+sx, y+row*scale+sy, r, g, b, a)
					}
				}
			}
		}
	}
	return 4 * scale
}

func drawNumber(x, y int32, text string, r, g, b, a uint8, scale int32) {
	// Shadow
	ox := x + 1
	for _, c := range text {
		ox += drawDigit(ox, y+1, c, 0, 0, 0, a/2, scale)
	}
	// Text
	ox = x
	for _, c := range text {
		ox += drawDigit(ox, y, c, r, g, b, a, scale)
	}
}

// ============================================================================
// RENDER
// ============================================================================
func getColor(t uint32) (uint8, uint8, uint8) {
	switch t {
	case TYPE_PLAYER:
		return 0, 255, 255 // Cyan
	case TYPE_NPC:
		return 255, 255, 0 // Yellow
	case TYPE_ITEM:
		return 0, 255, 0 // Green
	case TYPE_MOB:
		return 255, 0, 0 // Red
	case TYPE_PET:
		return 255, 150, 255 // Pink
	default:
		return 200, 200, 200
	}
}

func shouldShow(t uint32) bool {
	switch t {
	case TYPE_PLAYER:
		return showPlayers
	case TYPE_MOB:
		return showMobs
	case TYPE_ITEM:
		return showItems
	default:
		return false
	}
}

func render() {
	clearPixels()

	psx, psy := getPlayerScreenPos()
	if psx == 0 && psy == 0 {
		return
	}

	// Player crosshair - GREEN
	drawCross(psx, psy, 25, 0, 255, 0, 255, 4)

	actors := getAllActors()
	var mobCount, playerCount, itemCount int

	for _, a := range actors {
		switch a.Type {
		case TYPE_MOB:
			mobCount++
		case TYPE_PLAYER:
			playerCount++
		case TYPE_ITEM:
			itemCount++
		}

		if !shouldShow(a.Type) {
			continue
		}

		if a.ScreenX <= 0 || a.ScreenY <= 0 || a.ScreenX >= width || a.ScreenY >= height {
			continue
		}

		r, g, b := getColor(a.Type)

		// Box
		drawBox(a.ScreenX, a.ScreenY, 18, r, g, b, 255, 2)

		// Line
		if showLines {
			drawLine(psx, psy, a.ScreenX, a.ScreenY, r, g, b, 150, 1)
		}

		// Distance
		distText := fmt.Sprintf("%.0f", a.Distance)
		drawNumber(a.ScreenX-10, a.ScreenY-30, distText, r, g, b, 255, 2)
	}

	// Info panel background
	drawFilledRect(5, 5, 150, 90, 0, 0, 0, 180)

	// HP bar
	hp, maxHP := getPlayerHP()
	hpPct := float32(0)
	if maxHP > 0 {
		hpPct = float32(hp) / float32(maxHP)
	}
	drawFilledRect(10, 15, 130, 12, 50, 50, 50, 255)
	drawFilledRect(10, 15, int32(130*hpPct), 12, 255, 0, 0, 255)

	// SP bar
	sp, maxSP := getPlayerSP()
	spPct := float32(0)
	if maxSP > 0 {
		spPct = float32(sp) / float32(maxSP)
	}
	drawFilledRect(10, 32, 130, 12, 50, 50, 50, 255)
	drawFilledRect(10, 32, int32(130*spPct), 12, 0, 100, 255, 255)

	// Counts
	drawNumber(10, 55, fmt.Sprintf("%d", playerCount), 0, 255, 255, 255, 2)
	drawNumber(50, 55, fmt.Sprintf("%d", mobCount), 255, 0, 0, 255, 2)
	drawNumber(90, 55, fmt.Sprintf("%d", itemCount), 0, 255, 0, 255, 2)

	// Position
	wx, wy := getPlayerWorldPos()
	posText := fmt.Sprintf("%.0f-%.0f", wx, wy)
	drawNumber(10, 75, posText, 200, 200, 200, 255, 1)
}

func updatePosition() {
	var clientRect RECT
	procGetClientRect.Call(uintptr(gameHwnd), uintptr(unsafe.Pointer(&clientRect)))

	pt := POINT{0, 0}
	procClientToScreen.Call(uintptr(gameHwnd), uintptr(unsafe.Pointer(&pt)))

	newW := clientRect.Right
	newH := clientRect.Bottom

	if newW != width || newH != height && newW > 0 && newH > 0 {
		width = newW
		height = newH

		if memBitmap != 0 {
			procDeleteObject.Call(memBitmap)
		}

		bi := BITMAPINFO{
			Header: BITMAPINFOHEADER{
				Size:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
				Width:       width,
				Height:      height,
				Planes:      1,
				BitCount:    32,
				Compression: BI_RGB,
			},
		}

		memBitmap, _, _ = procCreateDIBSection.Call(
			memDC,
			uintptr(unsafe.Pointer(&bi)),
			DIB_RGB_COLORS,
			uintptr(unsafe.Pointer(&pixels)),
			0, 0,
		)
		procSelectObject.Call(memDC, memBitmap)
	}

	offsetX = pt.X
	offsetY = pt.Y
}

func updateOverlay() {
	render()

	screenDC, _, _ := procGetDC.Call(0)

	ptPos := POINT{offsetX, offsetY}
	ptSize := SIZE{width, height}
	ptSrc := POINT{0, 0}
	blend := BLENDFUNCTION{AC_SRC_OVER, 0, 255, AC_SRC_ALPHA}

	procUpdateLayeredWindow.Call(
		uintptr(overlayHwnd),
		screenDC,
		uintptr(unsafe.Pointer(&ptPos)),
		uintptr(unsafe.Pointer(&ptSize)),
		memDC,
		uintptr(unsafe.Pointer(&ptSrc)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ULW_ALPHA,
	)

	procReleaseDC.Call(0, screenDC)
}

func handleInput() {
	if isKeyPressed(VK_F1) {
		showPlayers = !showPlayers
		fmt.Printf("[ESP] Players: %v\n", showPlayers)
		time.Sleep(200 * time.Millisecond)
	}
	if isKeyPressed(VK_F2) {
		showMobs = !showMobs
		fmt.Printf("[ESP] Mobs: %v\n", showMobs)
		time.Sleep(200 * time.Millisecond)
	}
	if isKeyPressed(VK_F3) {
		showItems = !showItems
		fmt.Printf("[ESP] Items: %v\n", showItems)
		time.Sleep(200 * time.Millisecond)
	}
	if isKeyPressed(VK_F4) {
		showLines = !showLines
		fmt.Printf("[ESP] Lines: %v\n", showLines)
		time.Sleep(200 * time.Millisecond)
	}
	if isKeyPressed(VK_END) {
		running = false
	}
}

func isKeyPressed(vk int) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return ret&0x8000 != 0
}

func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == WM_DESTROY {
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// ============================================================================
// MAIN
// ============================================================================
func main() {
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║     RAGNAROK ONLINE ESP v2.0           ║")
	fmt.Println("╠════════════════════════════════════════╣")
	fmt.Println("║  Cyan   = Player                       ║")
	fmt.Println("║  Red    = Monster                      ║")
	fmt.Println("║  Green  = Item                         ║")
	fmt.Println("║  Yellow = NPC                          ║")
	fmt.Println("╠════════════════════════════════════════╣")
	fmt.Println("║  F1 = Toggle Players                   ║")
	fmt.Println("║  F2 = Toggle Monsters                  ║")
	fmt.Println("║  F3 = Toggle Items                     ║")
	fmt.Println("║  F4 = Toggle Lines                     ║")
	fmt.Println("║  END = Exit                            ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	var err error
	for _, name := range []string{"Ragnarok.exe", "ragnarok.exe", "RagexeRE.exe", "client.exe"} {
		roMem, err = NewMemory(name)
		if err == nil {
			fmt.Printf("[+] Found: %s (PID: %d)\n", name, roMem.pid)
			break
		}
	}
	if roMem == nil {
		fmt.Println("[-] Ragnarok not found!")
		fmt.Print("Press Enter to exit...")
		fmt.Scanln()
		return
	}
	defer roMem.Close()

	gameHwnd = findGameWindow(roMem.pid)
	if gameHwnd == 0 {
		fmt.Println("[-] Game window not found!")
		fmt.Scanln()
		return
	}
	fmt.Printf("[+] Window: 0x%X\n", gameHwnd)

	var clientRect RECT
	procGetClientRect.Call(uintptr(gameHwnd), uintptr(unsafe.Pointer(&clientRect)))
	width = clientRect.Right
	height = clientRect.Bottom
	if width == 0 {
		width = 1024
	}
	if height == 0 {
		height = 768
	}

	pt := POINT{0, 0}
	procClientToScreen.Call(uintptr(gameHwnd), uintptr(unsafe.Pointer(&pt)))
	offsetX = pt.X
	offsetY = pt.Y

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("ROESP_V2")

	wc := WNDCLASSEXW{
		Size:      uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:     CS_HREDRAW | CS_VREDRAW,
		WndProc:   syscall.NewCallback(wndProc),
		Instance:  syscall.Handle(hInstance),
		ClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowExW.Call(
		WS_EX_LAYERED|WS_EX_TRANSPARENT|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)), 0,
		WS_POPUP,
		uintptr(offsetX), uintptr(offsetY),
		uintptr(width), uintptr(height),
		0, 0, hInstance, 0,
	)
	overlayHwnd = syscall.Handle(hwnd)

	screenDC, _, _ := procGetDC.Call(0)
	memDC, _, _ = procCreateCompatibleDC.Call(screenDC)

	bi := BITMAPINFO{
		Header: BITMAPINFOHEADER{
			Size:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			Width:       width,
			Height:      height,
			Planes:      1,
			BitCount:    32,
			Compression: BI_RGB,
		},
	}

	memBitmap, _, _ = procCreateDIBSection.Call(
		memDC,
		uintptr(unsafe.Pointer(&bi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&pixels)),
		0, 0,
	)
	procSelectObject.Call(memDC, memBitmap)
	procReleaseDC.Call(0, screenDC)

	procShowWindow.Call(uintptr(overlayHwnd), SW_SHOW)

	fmt.Println("[+] ESP running!")
	fmt.Println()

	var msg MSG
	lastUpdate := time.Now()

	for running {
		for {
			ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, PM_REMOVE)
			if ret == 0 {
				break
			}
			if msg.Message == 0x0012 {
				running = false
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}

		handleInput()

		if time.Since(lastUpdate) >= 16*time.Millisecond {
			updatePosition()
			updateOverlay()
			lastUpdate = time.Now()
		}

		time.Sleep(1 * time.Millisecond)
	}

	if memBitmap != 0 {
		procDeleteObject.Call(memBitmap)
	}
	if memDC != 0 {
		procDeleteDC.Call(memDC)
	}

	fmt.Println("[+] Done!")
}