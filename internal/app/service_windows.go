package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/itousouta15/VencordGuard/internal/discord"
	"github.com/itousouta15/VencordGuard/internal/logging"
	"github.com/itousouta15/VencordGuard/internal/platform"
	"github.com/itousouta15/VencordGuard/internal/repair"
	"golang.org/x/sys/windows"
)

var ErrDiscordUpdating = errors.New("Discord is updating")

type Service struct {
	LocalAppData string
	Logger       *log.Logger
	Processes    platform.ProcessManager
	Downloader   *repair.Downloader
	locks        map[string]*sync.Mutex
}

func NewService(localAppData string, logger *log.Logger) *Service {
	locks := make(map[string]*sync.Mutex, len(discord.Channels))
	for _, channel := range discord.Channels {
		locks[channel.Branch] = &sync.Mutex{}
	}
	return &Service{
		LocalAppData: localAppData,
		Logger:       logger,
		Downloader:   repair.NewDownloader(localAppData),
		locks:        locks,
	}
}

func (s *Service) Status(channel discord.Channel) (discord.Install, error) {
	return discord.Discover(s.LocalAppData, channel)
}

func (s *Service) Ensure(ctx context.Context, channel discord.Channel, relaunch bool) (repaired bool, retErr error) {
	lock := s.locks[channel.Branch]
	lock.Lock()
	defer lock.Unlock()

	install, err := discord.Discover(s.LocalAppData, channel)
	if err != nil {
		return false, err
	}
	if install.Patch == discord.PatchHealthy {
		return false, nil
	}
	release, err := platform.AcquireRepairLock()
	if err != nil {
		return false, err
	}
	defer release()

	install, err = s.waitForUpdate(ctx, channel)
	if err != nil {
		return false, err
	}
	if install.Patch == discord.PatchHealthy {
		return false, nil
	}

	downloadCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	cli, expectedHash, err := s.Downloader.Installer(downloadCtx)
	cancel()
	if err != nil {
		return false, err
	}

	install, err = s.waitForUpdate(ctx, channel)
	if err != nil {
		return false, err
	}
	wasRunning, err := s.Processes.IsDiscordRunning(install)
	if err != nil {
		return false, err
	}
	stopped := false
	defer func() {
		if retErr == nil || !stopped || !wasRunning {
			return
		}
		latest, discoverErr := discord.Discover(s.LocalAppData, channel)
		if discoverErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("rediscover before recovery: %w", discoverErr))
			return
		}
		if launchErr := s.Processes.Launch(latest, false); launchErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("restart Discord after failed repair: %w", launchErr))
		}
	}()
	if wasRunning {
		s.Logger.Printf("stopping %s before repair", channel.Label)
		stopped, err = s.Processes.StopDiscord(install)
		if err != nil {
			return false, err
		}
	}
	updating, err := s.Processes.IsUpdateRunning(install)
	if err != nil {
		return false, err
	}
	if updating {
		return false, ErrDiscordUpdating
	}
	if err := repair.VerifyFile(cli, expectedHash); err != nil {
		return false, fmt.Errorf("reverify official installer: %w", err)
	}

	s.Logger.Printf("repairing %s with official Vencord Installer CLI", channel.Label)
	command := exec.Command(cli, "--repair", "--branch", channel.Branch)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	command.Stdout = logging.TailWriter(s.Logger, "Vencord CLI:")
	command.Stderr = logging.TailWriter(s.Logger, "Vencord CLI:")
	if err := command.Run(); err != nil {
		return false, fmt.Errorf("official Vencord repair failed: %w", err)
	}

	repairedInstall, err := discord.Discover(s.LocalAppData, channel)
	if err != nil {
		return false, err
	}
	if repairedInstall.Patch != discord.PatchHealthy {
		return false, fmt.Errorf("official installer completed but the patch is %s", repairedInstall.Patch)
	}
	if relaunch && wasRunning {
		stopped = false
		if err := s.Processes.Launch(repairedInstall, false); err != nil {
			time.Sleep(time.Second)
			if retryErr := s.Processes.Launch(repairedInstall, false); retryErr != nil {
				return true, errors.Join(
					fmt.Errorf("Vencord was repaired but Discord could not restart: %w", err),
					fmt.Errorf("Discord restart retry failed: %w", retryErr),
				)
			}
		}
	}
	s.Logger.Printf("%s repair completed", channel.Label)
	return true, nil
}

func (s *Service) waitForUpdate(ctx context.Context, channel discord.Channel) (discord.Install, error) {
	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		install, err := discord.Discover(s.LocalAppData, channel)
		if err != nil {
			return discord.Install{}, err
		}
		updating, err := s.Processes.IsUpdateRunning(install)
		if err != nil {
			return discord.Install{}, err
		}
		if !updating {
			return install, nil
		}
		select {
		case <-ctx.Done():
			return discord.Install{}, ctx.Err()
		case <-deadline.C:
			return discord.Install{}, ErrDiscordUpdating
		case <-ticker.C:
		}
	}
}

func (s *Service) Launch(ctx context.Context, channel discord.Channel, minimized bool) error {
	if _, err := s.Ensure(ctx, channel, false); err != nil {
		return err
	}
	install, err := discord.Discover(s.LocalAppData, channel)
	if err != nil {
		return err
	}
	return s.Processes.Launch(install, minimized)
}
