package repair

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.exe")
	data := []byte("official cli")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("%x", sha256.Sum256(data))
	if err := VerifyFile(path, expected); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("binary"))
	}))
	defer server.Close()
	d := &Downloader{Dir: t.TempDir(), Client: server.Client()}
	destination := filepath.Join(d.Dir, "download")
	if err := d.download(context.Background(), server.URL, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "binary" {
		t.Fatalf("got %q", data)
	}
}
