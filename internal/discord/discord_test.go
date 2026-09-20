package discord

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSelectsNewestNumericVersion(t *testing.T) {
	root := t.TempDir()
	channel := Channels[0]
	discordRoot := filepath.Join(root, channel.DirName)
	mustWrite(t, filepath.Join(discordRoot, "Update.exe"), []byte("update"))
	for _, version := range []string{"app-1.0.999", "app-1.0.1000"} {
		mustWrite(t, filepath.Join(discordRoot, version, channel.ExeName), []byte("exe"))
		mustWrite(t, filepath.Join(discordRoot, version, "resources", "app.asar"), []byte("asar"))
	}

	install, err := Discover(root, channel)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(install.AppDir); got != "app-1.0.1000" {
		t.Fatalf("selected %q", got)
	}
}

func TestInspectPatch(t *testing.T) {
	resources := filepath.Join(t.TempDir(), "resources")
	patcher := filepath.Join(t.TempDir(), "Vencord", "dist", "patcher.js")
	mustWrite(t, patcher, []byte("// Vencord test"))
	mustWrite(t, filepath.Join(resources, "_app.asar"), fakeStub(patcher))
	mustWrite(t, filepath.Join(resources, "app.asar"), fakeStub(patcher))

	if state := InspectPatch(resources); state != PatchHealthy {
		t.Fatalf("expected healthy, got %s", state)
	}
	if err := os.Remove(patcher); err != nil {
		t.Fatal(err)
	}
	if state := InspectPatch(resources); state != PatchBroken {
		t.Fatalf("expected broken, got %s", state)
	}
}

func fakeStub(patcher string) []byte {
	index := fmt.Sprintf(`require(%q)`, patcher)
	manifest := `{"name":"discord","main":"index.js"}`
	headerJSON := fmt.Sprintf(`{"files":{"index.js":{"size":%d,"offset":"0"},"package.json":{"size":%d,"offset":"%d"}}}`, len(index), len(manifest), len(index))
	headerStringSize := len(headerJSON)
	aligned := (headerStringSize + 3) &^ 3
	header := make([]byte, 16+aligned)
	binary.LittleEndian.PutUint32(header[0:4], 4)
	binary.LittleEndian.PutUint32(header[4:8], uint32(aligned+8))
	binary.LittleEndian.PutUint32(header[8:12], uint32(aligned+4))
	binary.LittleEndian.PutUint32(header[12:16], uint32(headerStringSize))
	copy(header[16:], headerJSON)
	return append(header, []byte(index+manifest)...)
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
