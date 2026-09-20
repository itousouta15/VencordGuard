package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	mutexName      = `Local\VencordGuard.UI.6854d047-9320-4c68-90eb-604f61312f75`
	repairLockName = `Local\VencordGuard.Repair.6854d047-9320-4c68-90eb-604f61312f75`
	quitName       = `Local\VencordGuard.Quit.6854d047-9320-4c68-90eb-604f61312f75`
	stoppedName    = `Local\VencordGuard.Stopped.6854d047-9320-4c68-90eb-604f61312f75`
)

func LocalAppData() (string, error) {
	value := os.Getenv("LOCALAPPDATA")
	if value == "" {
		return "", errors.New("LOCALAPPDATA is empty")
	}
	return filepath.Clean(value), nil
}

func IsTraditionalChinese() bool {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	value, _, _ := proc.Call()
	language := uint16(value)
	return language == 0x0404 || language == 0x0c04 || language == 0x1404 || language == 0x7c04
}

func Message(title, body string, errorIcon bool) {
	flags := uintptr(0x00000000 | 0x00001000)
	if errorIcon {
		flags |= 0x00000010
	} else {
		flags |= 0x00000040
	}
	titlePtr, _ := windows.UTF16PtrFromString(title)
	bodyPtr, _ := windows.UTF16PtrFromString(body)
	user32 := windows.NewLazySystemDLL("user32.dll")
	_, _, _ = user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(bodyPtr)), uintptr(unsafe.Pointer(titlePtr)), flags)
}

func AcquireGuardInstance() (func(), bool, error) {
	name, _ := windows.UTF16PtrFromString(mutexName)
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, false, err
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(handle)
		return func() {}, false, nil
	}
	return func() { windows.CloseHandle(handle) }, true, nil
}

func CreateQuitEvent() (windows.Handle, error) {
	name, _ := windows.UTF16PtrFromString(quitName)
	handle, err := windows.CreateEvent(nil, 1, 0, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		err = nil
	}
	if err == nil {
		_ = windows.ResetEvent(handle)
	}
	return handle, err
}

func CreateStoppedEvent() (windows.Handle, error) {
	name, _ := windows.UTF16PtrFromString(stoppedName)
	handle, err := windows.CreateEvent(nil, 1, 0, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		err = nil
	}
	if err == nil {
		_ = windows.ResetEvent(handle)
	}
	return handle, err
}

func SignalQuit() error {
	name, _ := windows.UTF16PtrFromString(quitName)
	handle, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return nil
		}
		return err
	}
	defer windows.CloseHandle(handle)
	if err := windows.SetEvent(handle); err != nil {
		return err
	}
	stopped, err := windows.OpenEvent(windows.SYNCHRONIZE, false, windows.StringToUTF16Ptr(stoppedName))
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(stopped)
	result, err := windows.WaitForSingleObject(stopped, 10*60*1000)
	if err != nil {
		return err
	}
	if result != windows.WAIT_OBJECT_0 {
		return errors.New("timed out waiting for VencordGuard to stop")
	}
	return nil
}

func AcquireRepairLock() (func(), error) {
	runtime.LockOSThread()
	name, _ := windows.UTF16PtrFromString(repairLockName)
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		runtime.UnlockOSThread()
		return nil, err
	}
	result, err := windows.WaitForSingleObject(handle, 5*60*1000)
	if err != nil {
		windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, err
	}
	if result != windows.WAIT_OBJECT_0 && result != windows.WAIT_ABANDONED {
		windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, errors.New("timed out waiting for another Vencord repair")
	}
	return func() {
		_ = windows.ReleaseMutex(handle)
		windows.CloseHandle(handle)
		runtime.UnlockOSThread()
	}, nil
}

func StartupEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue("VencordGuard")
	return err == nil
}

func SetStartup(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !enabled {
		err := key.DeleteValue("VencordGuard")
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := fmt.Sprintf("\"%s\" --guard", strings.ReplaceAll(executable, "\"", ""))
	return key.SetStringValue("VencordGuard", command)
}
