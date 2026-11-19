package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procCreateIconFromResEx = user32.NewProc("CreateIconFromResourceEx")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wmTrayCallback = 0x0400 + 42
	wmCommand      = 0x0111
	wmLButtonUp    = 0x0202
	wmLButtonDbl   = 0x0203
	wmRButtonUp    = 0x0205

	nimAdd    = 0x00000000
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	tpmBottom   = 0x0020
	tpmLeft     = 0x0000

	cmdTrayOpen  = 1001
	cmdTrayStart = 1002
	cmdTrayStop  = 1003
	cmdTrayExit  = 1004
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type notifyIconDataW struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type point struct {
	X, Y int32
}

type msgStruct struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

var trayHwnd uintptr
var trayNID notifyIconDataW

func removeTrayIcon() {
	if trayHwnd != 0 {
		_, _, _ = procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&trayNID)))
	}
}

func showTrayContextMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	tOpen, _ := syscall.UTF16PtrFromString("Открыть Quest Tracker")
	tStart, _ := syscall.UTF16PtrFromString("Запустить все квесты")
	tStop, _ := syscall.UTF16PtrFromString("Остановить")
	tExit, _ := syscall.UTF16PtrFromString("Выход")

	_, _, _ = procAppendMenuW.Call(hMenu, mfString, cmdTrayOpen, uintptr(unsafe.Pointer(tOpen)))
	_, _, _ = procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	_, _, _ = procAppendMenuW.Call(hMenu, mfString, cmdTrayStart, uintptr(unsafe.Pointer(tStart)))
	_, _, _ = procAppendMenuW.Call(hMenu, mfString, cmdTrayStop, uintptr(unsafe.Pointer(tStop)))
	_, _, _ = procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	_, _, _ = procAppendMenuW.Call(hMenu, mfString, cmdTrayExit, uintptr(unsafe.Pointer(tExit)))

	var pt point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	_, _, _ = procSetForegroundWindow.Call(hwnd)
	_, _, _ = procTrackPopupMenu.Call(hMenu, tpmBottom|tpmLeft, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
}

func trayWndProc(hwnd uintptr, uMsg uint32, wParam, lParam uintptr) uintptr {
	switch uMsg {
	case wmTrayCallback:
		switch uint32(lParam) {
		case wmLButtonUp, wmLButtonDbl:
			setHiddenToTray(false)
			openUIWindow("http://" + serverAddr)
		case wmRButtonUp:
			showTrayContextMenu(hwnd)
		}
		return 0
	case wmCommand:
		switch uint16(wParam & 0xFFFF) {
		case cmdTrayOpen:
			setHiddenToTray(false)
			openUIWindow("http://" + serverAddr)
		case cmdTrayStart:
			_, _ = evalInDiscord("window.__questTrackerV9 && window.__questTrackerV9.startAll()")
		case cmdTrayStop:
			_, _ = evalInDiscord("window.__questTrackerV9 && window.__questTrackerV9.stop()")
		case cmdTrayExit:
			shutdownAppCompletely()
		}
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(uMsg), wParam, lParam)
	return ret
}

func startSystemTray(pngIcon []byte) {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		hInst, _, _ := procGetModuleHandleW.Call(0)
		className, _ := syscall.UTF16PtrFromString("KastrixQuestTrackerTrayClass")

		wc := wndClassExW{
			cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
			lpfnWndProc:   syscall.NewCallback(trayWndProc),
			hInstance:     hInst,
			lpszClassName: className,
		}
		_, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

		hwnd, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(className)),
			0, 0, 0, 0, 0,
			0, 0, hInst, 0,
		)
		if hwnd == 0 {
			return
		}
		trayHwnd = hwnd

		var hIcon uintptr
		if len(pngIcon) > 0 {
			hIcon, _, _ = procCreateIconFromResEx.Call(
				uintptr(unsafe.Pointer(&pngIcon[0])),
				uintptr(len(pngIcon)),
				1,
				0x00030000,
				32, 32,
				0,
			)
		}

		trayNID = notifyIconDataW{
			cbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
			hWnd:             hwnd,
			uID:              1,
			uFlags:           nifMessage | nifIcon | nifTip,
			uCallbackMessage: wmTrayCallback,
			hIcon:            hIcon,
		}
		tip, _ := syscall.UTF16FromString("Kastrix — Discord Quest Tracker")
		copy(trayNID.szTip[:], tip)

		_, _, _ = procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&trayNID)))

		var m msgStruct
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(ret) <= 0 {
				break
			}
			_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
}
