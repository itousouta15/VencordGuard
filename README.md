# VencordGuard

VencordGuard checks whether [Vencord](https://vencord.dev/) is still injected whenever Discord starts or updates. If Discord replaced the injection, VencordGuard repairs it with the official Vencord Installer CLI and then starts Discord normally.

VencordGuard 會在 Discord 啟動或更新後檢查 [Vencord](https://vencord.dev/) 是否仍正常套用。如果 Discord 更新覆蓋了注入內容，它會透過 Vencord 官方安裝器 CLI 修復，再正常啟動 Discord。

> [!IMPORTANT]
> VencordGuard is an independent community project. It is not affiliated with Discord or Vencord. Client modifications are against Discord's Terms of Service; use them at your own risk.
>
> VencordGuard 是獨立的社群專案，與 Discord、Vencord 官方皆無關。修改 Discord 用戶端可能違反 Discord 服務條款，請自行承擔使用風險。

## Features / 功能

- Supports Discord Stable, PTB, and Canary.
- 提供背景守護與專用啟動捷徑兩種模式。
- Validates the complete patch instead of checking only one marker file.
- Only downloads `VencordInstallerCli.exe` from the official Vencord GitHub release.
- Requires the SHA-256 digest published by GitHub before running the downloaded CLI.
- Runs per user and does not require administrator privileges.
- Uses the Windows display language for Traditional Chinese or English messages.
- Contains no telemetry and collects no personal data.

## Install / 安裝

Download `VencordGuard-Setup.exe` from the latest [GitHub Release](../../releases/latest). The installer offers two independent components:

- **Background guard / 背景守護**: starts with Windows and monitors every installed Discord channel. This covers Discord's existing Start menu and taskbar shortcuts.
- **Guarded shortcuts / 專用捷徑**: creates separate launchers that check and repair Vencord before opening Discord. Nothing stays resident after Discord starts.

Both can be installed together. A portable ZIP is also available and supports the same command-line modes.

安裝程式可同時啟用兩種模式。背景守護能涵蓋原本的 Discord 捷徑；專用捷徑模式則不會常駐。Release 也提供免安裝 ZIP。

Windows SmartScreen may warn about early unsigned releases. Verify the file against `SHA256SUMS.txt` if this occurs. The project cannot establish SmartScreen reputation until releases are code-signed.

## Commands / 命令

```text
VencordGuard.exe --guard
VencordGuard.exe --launch stable|ptb|canary
VencordGuard.exe --repair stable|ptb|canary
VencordGuard.exe --status
VencordGuard.exe --startup enable|disable
VencordGuard.exe --quit
VencordGuard.exe --version
```

## How It Works / 運作方式

1. VencordGuard discovers the newest complete `app-*` directory for each Discord channel.
2. It verifies `_app.asar`, the injected `app.asar` stub, its `require(...)` target, and the target `patcher.js`.
3. When repair is needed, it waits up to two minutes for Discord Update to finish and stops only Discord processes running below that channel's installation directory.
4. It obtains the latest official CLI metadata from `Vencord/Installer`, verifies the asset's published SHA-256 digest, and runs `--repair --branch <channel>`.
5. It validates the patch again and restarts Discord only if Discord was running before repair.

The guard scans three small local directories every two seconds. It requires two consecutive unhealthy checks and applies a retry delay after errors, preventing update races and restart loops. A guarded shortcut performs the same check before starting Discord; background mode provides eventual protection for existing shortcuts.

## Data and Files / 資料與檔案

VencordGuard stores only the official CLI cache, its digest metadata, and a local diagnostic log:

```text
%LOCALAPPDATA%\VencordGuard\tools
%LOCALAPPDATA%\VencordGuard\logs\VencordGuard.log
```

It does not send logs or usage information anywhere. Uninstalling the app does not remove Discord, Vencord, themes, or Vencord settings.

## Build

Requirements: Go 1.26 or newer. Inno Setup 6 is additionally required to build the installer.

```powershell
go test ./...
go vet ./...
go generate ./cmd/vencordguard
go build -trimpath -ldflags "-H=windowsgui -s -w -X main.version=dev" -o build/VencordGuard.exe ./cmd/vencordguard
```

Tagged pushes build the portable ZIP and installer through GitHub Actions.

## License

VencordGuard is licensed under GPL-3.0. Vencord and the Vencord Installer are separate GPL-3.0 projects maintained by their respective contributors.
