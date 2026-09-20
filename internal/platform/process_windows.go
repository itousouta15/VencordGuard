package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/itousouta15/VencordGuard/internal/discord"
	"golang.org/x/sys/windows"
)

type ProcessManager struct{}

func (ProcessManager) IsUpdateRunning(install discord.Install) (bool, error) {
	processes, err := matchingProcesses("Update.exe", install.Root)
	return len(processes) > 0, err
}

func (ProcessManager) IsDiscordRunning(install discord.Install) (bool, error) {
	processes, err := matchingProcesses(install.Channel.ExeName, install.Root)
	return len(processes) > 0, err
}

func (ProcessManager) StopDiscord(install discord.Install) (bool, error) {
	stopped := false
	for attempt := 0; attempt < 10; attempt++ {
		processes, err := matchingProcesses(install.Channel.ExeName, install.Root)
		if err != nil {
			return stopped, err
		}
		if len(processes) == 0 {
			return stopped, nil
		}
		stopped = true
		pids := make(map[uint32]bool, len(processes))
		for _, process := range processes {
			pids[process.pid] = true
		}
		for _, process := range processes {
			if pids[process.parentPID] {
				continue
			}
			path, err := processPath(process.pid)
			if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
				continue
			}
			if err != nil {
				return true, fmt.Errorf("revalidate %s process %d: %w", install.Channel.Label, process.pid, err)
			}
			root := strings.TrimSuffix(filepath.Clean(install.Root), string(filepath.Separator)) + string(filepath.Separator)
			if !hasPathPrefix(path, root) {
				continue
			}
			command := exec.Command("taskkill.exe", "/PID", strconv.FormatUint(uint64(process.pid), 10), "/T", "/F")
			command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
			_ = command.Run()
		}
		time.Sleep(250 * time.Millisecond)
	}
	return stopped, fmt.Errorf("could not stop all %s processes", install.Channel.Label)
}

func (ProcessManager) Launch(install discord.Install, minimized bool) error {
	args := []string{"--processStart", install.Channel.ExeName}
	if minimized {
		args = append(args, "--process-start-args", "--start-inactive --start-minimized")
	}
	command := exec.Command(install.UpdateExe, args...)
	command.Dir = install.Root
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

type processInfo struct {
	pid       uint32
	parentPID uint32
	path      string
}

func matchingProcesses(exeName, root string) ([]processInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, err
	}

	cleanRoot := strings.TrimSuffix(filepath.Clean(root), string(filepath.Separator)) + string(filepath.Separator)
	var found []processInfo
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, exeName) {
			path, err := processPath(entry.ProcessID)
			if err != nil {
				if errors.Is(err, windows.ERROR_INVALID_PARAMETER) || (errors.Is(err, windows.ERROR_ACCESS_DENIED) && strings.EqualFold(exeName, "Update.exe")) {
					goto next
				}
				return nil, fmt.Errorf("inspect process %s (%d): %w", name, entry.ProcessID, err)
			}
			if hasPathPrefix(path, cleanRoot) {
				found = append(found, processInfo{pid: entry.ProcessID, parentPID: entry.ParentProcessID, path: path})
			}
		}
	next:
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, err
		}
	}
	return found, nil
}

func processPath(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 32_768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

func hasPathPrefix(path, root string) bool {
	path = filepath.Clean(path)
	return len(path) >= len(root) && strings.EqualFold(path[:len(root)], root)
}
