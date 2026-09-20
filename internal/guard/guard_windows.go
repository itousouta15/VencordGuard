package guard

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/itousouta15/VencordGuard/internal/app"
	"github.com/itousouta15/VencordGuard/internal/discord"
	"github.com/itousouta15/VencordGuard/internal/i18n"
	"github.com/itousouta15/VencordGuard/internal/platform"
	"github.com/lxn/walk"
	"golang.org/x/sys/windows"
)

type tray struct {
	service     *app.Service
	text        i18n.Text
	logger      *log.Logger
	window      *walk.MainWindow
	notify      *walk.NotifyIcon
	statuses    map[string]*walk.Action
	startup     *walk.Action
	ctx         context.Context
	cancel      context.CancelFunc
	repairMutex sync.Mutex
	workerMutex sync.Mutex
	workers     sync.WaitGroup
	closing     bool
}

func Run(service *app.Service, text i18n.Text, logger *log.Logger) error {
	release, acquired, err := platform.AcquireGuardInstance()
	if err != nil {
		return err
	}
	if !acquired {
		platform.Message(text.AppTitle, text.AlreadyRunning, false)
		return nil
	}
	defer release()

	quitEvent, err := platform.CreateQuitEvent()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(quitEvent)
	stoppedEvent, err := platform.CreateStoppedEvent()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(stoppedEvent)
	defer windows.SetEvent(stoppedEvent)

	window, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	window.SetVisible(false)
	notify, err := walk.NewNotifyIcon(window)
	if err != nil {
		return err
	}
	defer notify.Dispose()
	if err := notify.SetIcon(walk.IconShield()); err != nil {
		return err
	}
	if err := notify.SetToolTip(text.AppTitle); err != nil {
		return err
	}
	if err := notify.SetVisible(true); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &tray{
		service:  service,
		text:     text,
		logger:   logger,
		window:   window,
		notify:   notify,
		statuses: make(map[string]*walk.Action),
		ctx:      ctx,
		cancel:   cancel,
	}
	defer cancel()
	if err := t.buildMenu(); err != nil {
		return err
	}

	t.startWorker(t.monitor)
	go func() {
		_, _ = windows.WaitForSingleObject(quitEvent, windows.INFINITE)
		t.shutdown()
	}()
	window.Run()
	cancel()
	t.workers.Wait()
	return nil
}

func (t *tray) buildMenu() error {
	for _, channel := range discord.Channels {
		action := walk.NewAction()
		if err := action.SetText(fmt.Sprintf("%s: %s", channel.Label, t.text.Checking)); err != nil {
			return err
		}
		action.SetEnabled(false)
		if err := t.notify.ContextMenu().Actions().Add(action); err != nil {
			return err
		}
		t.statuses[channel.Branch] = action
	}
	if err := t.notify.ContextMenu().Actions().Add(walk.NewSeparatorAction()); err != nil {
		return err
	}

	repairAll := walk.NewAction()
	_ = repairAll.SetText(t.text.RepairAll)
	repairAll.Triggered().Attach(func() { t.startWorker(t.repairAll) })
	_ = t.notify.ContextMenu().Actions().Add(repairAll)

	labels := []string{t.text.LaunchStable, t.text.LaunchPTB, t.text.LaunchCanary}
	for index, channel := range discord.Channels {
		channel := channel
		action := walk.NewAction()
		_ = action.SetText(labels[index])
		action.Triggered().Attach(func() {
			t.startWorker(func() {
				if err := t.service.Launch(t.ctx, channel, false); err != nil {
					t.showError(channel, err)
				}
			})
		})
		_ = t.notify.ContextMenu().Actions().Add(action)
	}
	_ = t.notify.ContextMenu().Actions().Add(walk.NewSeparatorAction())

	t.startup = walk.NewAction()
	_ = t.startup.SetText(t.text.StartWithWindows)
	t.startup.SetCheckable(true)
	_ = t.startup.SetChecked(platform.StartupEnabled())
	t.startup.Triggered().Attach(func() {
		enabled := t.startup.Checked()
		if err := platform.SetStartup(enabled); err != nil {
			_ = t.startup.SetChecked(!enabled)
			t.logger.Printf("ERROR: startup setting: %v", err)
			_ = t.notify.ShowError(t.text.RepairFailedTitle, err.Error())
		}
	})
	_ = t.notify.ContextMenu().Actions().Add(t.startup)

	openLogs := walk.NewAction()
	_ = openLogs.SetText(t.text.OpenLogs)
	openLogs.Triggered().Attach(func() {
		path := filepath.Join(t.service.LocalAppData, "VencordGuard", "logs")
		_ = exec.Command("explorer.exe", path).Start()
	})
	_ = t.notify.ContextMenu().Actions().Add(openLogs)

	exit := walk.NewAction()
	_ = exit.SetText(t.text.Exit)
	exit.Triggered().Attach(t.shutdown)
	return t.notify.ContextMenu().Actions().Add(exit)
}

func (t *tray) startWorker(work func()) {
	t.workerMutex.Lock()
	if t.closing {
		t.workerMutex.Unlock()
		return
	}
	t.workers.Add(1)
	t.workerMutex.Unlock()
	go func() {
		defer t.workers.Done()
		work()
	}()
}

func (t *tray) shutdown() {
	t.workerMutex.Lock()
	if t.closing {
		t.workerMutex.Unlock()
		return
	}
	t.closing = true
	t.workerMutex.Unlock()
	t.cancel()
	go func() {
		t.workers.Wait()
		t.window.Synchronize(func() { walk.App().Exit(0) })
	}()
}

func (t *tray) monitor() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	unhealthy := make(map[string]int)
	backoff := make(map[string]time.Time)
	for {
		t.scan(unhealthy, backoff)
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (t *tray) scan(unhealthy map[string]int, backoff map[string]time.Time) {
	for _, channel := range discord.Channels {
		install, err := t.service.Status(channel)
		if err != nil {
			if discord.IsNotInstalled(err) {
				t.setStatus(channel, t.text.NotInstalled)
			} else {
				t.logger.Printf("ERROR: inspect %s: %v", channel.Label, err)
				t.setStatus(channel, t.text.RepairNeeded)
			}
			unhealthy[channel.Branch] = 0
			continue
		}
		if install.Patch == discord.PatchHealthy {
			t.setStatus(channel, t.text.Protected)
			unhealthy[channel.Branch] = 0
			continue
		}
		t.setStatus(channel, t.text.RepairNeeded)
		unhealthy[channel.Branch]++
		if unhealthy[channel.Branch] < 2 || time.Now().Before(backoff[channel.Branch]) {
			continue
		}
		repaired, err := t.service.Ensure(t.ctx, channel, true)
		unhealthy[channel.Branch] = 0
		if errors.Is(err, app.ErrDiscordUpdating) {
			continue
		}
		if err != nil {
			backoff[channel.Branch] = time.Now().Add(time.Minute)
			t.showError(channel, err)
			continue
		}
		if repaired {
			t.showSuccess(channel)
		}
	}
}

func (t *tray) repairAll() {
	t.repairMutex.Lock()
	defer t.repairMutex.Unlock()
	for _, channel := range discord.Channels {
		if _, err := t.service.Status(channel); err != nil {
			continue
		}
		repaired, err := t.service.Ensure(t.ctx, channel, true)
		if err != nil {
			t.showError(channel, err)
		} else if repaired {
			t.showSuccess(channel)
		}
	}
}

func (t *tray) setStatus(channel discord.Channel, status string) {
	t.window.Synchronize(func() {
		_ = t.statuses[channel.Branch].SetText(fmt.Sprintf("%s: %s", channel.Label, status))
	})
}

func (t *tray) showSuccess(channel discord.Channel) {
	t.logger.Printf("notification: %s repaired", channel.Label)
	t.window.Synchronize(func() {
		_ = t.notify.ShowInfo(t.text.RepairSuccessTitle, fmt.Sprintf(t.text.RepairSuccessBody, channel.Label))
	})
}

func (t *tray) showError(channel discord.Channel, err error) {
	t.logger.Printf("ERROR: %s: %v", channel.Label, err)
	t.window.Synchronize(func() {
		_ = t.notify.ShowError(t.text.RepairFailedTitle, fmt.Sprintf(t.text.RepairFailedBody, channel.Label, err))
	})
}
