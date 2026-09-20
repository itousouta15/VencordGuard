package main

//go:generate go run github.com/akavel/rsrc@v0.10.2 -manifest vencordguard.manifest -o rsrc_windows_amd64.syso

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/itousouta15/VencordGuard/internal/app"
	"github.com/itousouta15/VencordGuard/internal/discord"
	"github.com/itousouta15/VencordGuard/internal/guard"
	"github.com/itousouta15/VencordGuard/internal/i18n"
	"github.com/itousouta15/VencordGuard/internal/logging"
	"github.com/itousouta15/VencordGuard/internal/platform"
)

var version = "dev"

func main() {
	runtime.LockOSThread()
	localAppData, err := platform.LocalAppData()
	if err != nil {
		platform.Message("VencordGuard", err.Error(), true)
		return
	}
	logger, closeLog, err := logging.New(localAppData)
	if err != nil {
		platform.Message("VencordGuard", err.Error(), true)
		return
	}
	defer closeLog()
	logger.Printf("starting VencordGuard %s: %q", version, os.Args[1:])

	text := i18n.ForTraditionalChinese(platform.IsTraditionalChinese())
	service := app.NewService(localAppData, logger)
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "--guard" {
		if err := guard.Run(service, text, logger); err != nil {
			logger.Printf("ERROR: guard: %v", err)
			platform.Message(text.AppTitle, err.Error(), true)
		}
		return
	}

	switch args[0] {
	case "--launch":
		channel, ok := parseChannel(args)
		if !ok {
			showUsage(text)
			return
		}
		if err := service.Launch(context.Background(), channel, false); err != nil {
			showCommandError(text, channel, err)
		}
	case "--repair":
		channel, ok := parseChannel(args)
		if !ok {
			showUsage(text)
			return
		}
		repaired, err := service.Ensure(context.Background(), channel, true)
		if err != nil {
			showCommandError(text, channel, err)
		} else if repaired {
			platform.Message(text.RepairSuccessTitle, fmt.Sprintf(text.RepairSuccessBody, channel.Label), false)
		}
	case "--status":
		showStatus(service, text)
	case "--quit":
		if err := platform.SignalQuit(); err != nil {
			platform.Message(text.AppTitle, err.Error(), true)
		}
	case "--startup":
		if len(args) != 2 || (args[1] != "enable" && args[1] != "disable") {
			showUsage(text)
			return
		}
		if err := platform.SetStartup(args[1] == "enable"); err != nil {
			platform.Message(text.AppTitle, err.Error(), true)
		}
	case "--version":
		platform.Message(text.AppTitle, "VencordGuard "+version, false)
	default:
		showUsage(text)
	}
}

func parseChannel(args []string) (discord.Channel, bool) {
	if len(args) != 2 {
		return discord.Channel{}, false
	}
	return discord.ChannelByBranch(strings.ToLower(args[1]))
}

func showCommandError(text i18n.Text, channel discord.Channel, err error) {
	if discord.IsNotInstalled(err) {
		platform.Message(text.AppTitle, fmt.Sprintf(text.DiscordNotInstalled, channel.Label), true)
		return
	}
	platform.Message(text.RepairFailedTitle, fmt.Sprintf(text.RepairFailedBody, channel.Label, err), true)
}

func showStatus(service *app.Service, text i18n.Text) {
	var lines []string
	for _, channel := range discord.Channels {
		install, err := service.Status(channel)
		if err != nil {
			if discord.IsNotInstalled(err) {
				lines = append(lines, channel.Label+": "+text.NotInstalled)
			} else {
				lines = append(lines, channel.Label+": "+err.Error())
			}
			continue
		}
		status := text.RepairNeeded
		if install.Patch == discord.PatchHealthy {
			status = text.Protected
		}
		lines = append(lines, channel.Label+": "+status)
	}
	platform.Message(text.AppTitle, strings.Join(lines, "\n"), false)
}

func showUsage(text i18n.Text) {
	message := "Usage / 用法:\n\n" +
		"VencordGuard.exe --guard\n" +
		"VencordGuard.exe --launch stable|ptb|canary\n" +
		"VencordGuard.exe --repair stable|ptb|canary\n" +
		"VencordGuard.exe --status\n" +
		"VencordGuard.exe --quit"
	platform.Message(text.AppTitle, message, true)
}
