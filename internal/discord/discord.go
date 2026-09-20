package discord

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Channel struct {
	Branch  string
	DirName string
	ExeName string
	Label   string
}

var Channels = []Channel{
	{Branch: "stable", DirName: "Discord", ExeName: "Discord.exe", Label: "Discord Stable"},
	{Branch: "ptb", DirName: "DiscordPTB", ExeName: "DiscordPTB.exe", Label: "Discord PTB"},
	{Branch: "canary", DirName: "DiscordCanary", ExeName: "DiscordCanary.exe", Label: "Discord Canary"},
}

type Install struct {
	Channel   Channel
	Root      string
	AppDir    string
	Resources string
	UpdateExe string
	MainExe   string
	Patch     PatchState
}

type PatchState int

const (
	PatchUnknown PatchState = iota
	PatchAbsent
	PatchHealthy
	PatchBroken
)

func (p PatchState) String() string {
	switch p {
	case PatchAbsent:
		return "not patched"
	case PatchHealthy:
		return "protected"
	case PatchBroken:
		return "broken patch"
	default:
		return "unknown"
	}
}

func ChannelByBranch(branch string) (Channel, bool) {
	for _, channel := range Channels {
		if channel.Branch == strings.ToLower(branch) {
			return channel, true
		}
	}
	return Channel{}, false
}

func Discover(localAppData string, channel Channel) (Install, error) {
	root := filepath.Join(localAppData, channel.DirName)
	updateExe := filepath.Join(root, "Update.exe")
	if !regularFile(updateExe) {
		return Install{}, os.ErrNotExist
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return Install{}, err
	}

	var best string
	var bestVersion []int
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(strings.ToLower(entry.Name()), "app-") {
			continue
		}
		version, ok := parseVersion(entry.Name()[4:])
		if !ok {
			continue
		}
		appDir := filepath.Join(root, entry.Name())
		resources := filepath.Join(appDir, "resources")
		mainExe := filepath.Join(appDir, channel.ExeName)
		if !regularFile(mainExe) || !directory(resources) || !regularFile(filepath.Join(resources, "app.asar")) {
			continue
		}
		if best == "" || compareVersion(version, bestVersion) > 0 {
			best = appDir
			bestVersion = version
		}
	}

	if best == "" {
		return Install{}, fmt.Errorf("%s installation is incomplete", channel.Label)
	}
	resources := filepath.Join(best, "resources")
	return Install{
		Channel:   channel,
		Root:      root,
		AppDir:    best,
		Resources: resources,
		UpdateExe: updateExe,
		MainExe:   filepath.Join(best, channel.ExeName),
		Patch:     InspectPatch(resources),
	}, nil
}

func DiscoverAll(localAppData string) []Install {
	installs := make([]Install, 0, len(Channels))
	for _, channel := range Channels {
		install, err := Discover(localAppData, channel)
		if err == nil {
			installs = append(installs, install)
		}
	}
	return installs
}

var requirePattern = regexp.MustCompile(`require\(("(?:\\.|[^"\\])*")\)`)

func InspectPatch(resources string) PatchState {
	original := filepath.Join(resources, "_app.asar")
	if !regularFile(original) {
		return PatchAbsent
	}
	if !validAsarEnvelope(original) {
		return PatchBroken
	}

	stub := filepath.Join(resources, "app.asar")
	info, err := os.Stat(stub)
	if err != nil || info.Size() <= 0 || info.Size() > 1024*1024 {
		return PatchBroken
	}
	index, packageJSON, err := readStub(stub)
	if err != nil {
		return PatchBroken
	}
	var manifest struct {
		Name string `json:"name"`
		Main string `json:"main"`
	}
	if json.Unmarshal(packageJSON, &manifest) != nil || manifest.Name != "discord" || manifest.Main != "index.js" {
		return PatchBroken
	}
	match := requirePattern.FindSubmatch(index)
	if len(match) != 2 {
		return PatchBroken
	}
	patcherPath, err := strconv.Unquote(string(match[1]))
	if err != nil || !strings.EqualFold(filepath.Base(patcherPath), "patcher.js") || !validPatcher(patcherPath) {
		return PatchBroken
	}
	return PatchHealthy
}

type asarEntry struct {
	Size   int64  `json:"size"`
	Offset string `json:"offset"`
}

type asarHeader struct {
	Files map[string]asarEntry `json:"files"`
}

func readStub(path string) ([]byte, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 16 {
		return nil, nil, errors.New("invalid ASAR stub")
	}
	headerLength := int(binary.LittleEndian.Uint32(data[12:16]))
	alignedLength := (headerLength + 3) &^ 3
	if headerLength <= 1 || 16+alignedLength > len(data) {
		return nil, nil, errors.New("invalid ASAR header length")
	}
	var header asarHeader
	if err := json.Unmarshal(data[16:16+headerLength], &header); err != nil {
		return nil, nil, err
	}
	dataStart := 16 + alignedLength
	index, err := asarFile(data, dataStart, header.Files["index.js"])
	if err != nil {
		return nil, nil, err
	}
	manifest, err := asarFile(data, dataStart, header.Files["package.json"])
	if err != nil {
		return nil, nil, err
	}
	return index, manifest, nil
}

func asarFile(data []byte, dataStart int, entry asarEntry) ([]byte, error) {
	offset, err := strconv.Atoi(entry.Offset)
	if err != nil || offset < 0 || entry.Size <= 0 || entry.Size > int64(len(data)) {
		return nil, errors.New("invalid ASAR file entry")
	}
	start := dataStart + offset
	end := start + int(entry.Size)
	if start < dataStart || end < start || end > len(data) {
		return nil, errors.New("ASAR file entry is out of bounds")
	}
	return data[start:end], nil
}

func validAsarEnvelope(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() < 16 {
		return false
	}
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return false
	}
	dataSize := binary.LittleEndian.Uint32(header[0:4])
	headerSize := binary.LittleEndian.Uint32(header[4:8])
	headerObjectSize := binary.LittleEndian.Uint32(header[8:12])
	headerStringSize := binary.LittleEndian.Uint32(header[12:16])
	aligned := (uint64(headerStringSize) + 3) &^ 3
	return dataSize == 4 && headerStringSize > 1 &&
		headerObjectSize >= headerStringSize+4 && headerSize >= headerObjectSize+4 &&
		16+aligned <= uint64(info.Size())
}

func validPatcher(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	data := make([]byte, 128)
	n, err := file.Read(data)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return n > 11 && strings.HasPrefix(string(data[:n]), "// Vencord ")
}

func parseVersion(raw string) ([]int, bool) {
	parts := strings.Split(raw, ".")
	if len(parts) == 0 {
		return nil, false
	}
	version := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			return nil, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		version[i] = n
	}
	return version, true
}

func compareVersion(a, b []int) int {
	length := len(a)
	if len(b) > length {
		length = len(b)
	}
	for i := 0; i < length; i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func directory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func IsNotInstalled(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
