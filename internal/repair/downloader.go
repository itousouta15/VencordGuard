package repair

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	releaseURL = "https://api.github.com/repos/Vencord/Installer/releases/latest"
	assetName  = "VencordInstallerCli.exe"
)

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
		Digest      string `json:"digest"`
	} `json:"assets"`
}

type cacheMetadata struct {
	Tag    string `json:"tag"`
	SHA256 string `json:"sha256"`
}

type Downloader struct {
	Dir    string
	Client *http.Client
}

func NewDownloader(localAppData string) *Downloader {
	return &Downloader{
		Dir: filepath.Join(localAppData, "VencordGuard", "tools"),
		Client: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (d *Downloader) Installer(ctx context.Context) (string, string, error) {
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		return "", "", err
	}
	installerPath := filepath.Join(d.Dir, assetName)
	metadataPath := filepath.Join(d.Dir, "installer.json")

	latest, err := d.fetchRelease(ctx)
	if err != nil {
		return "", "", fmt.Errorf("fetch Vencord Installer release: %w", err)
	}

	var downloadURL, expectedHash string
	for _, asset := range latest.Assets {
		if asset.Name == assetName {
			downloadURL = asset.DownloadURL
			expectedHash = strings.TrimPrefix(strings.ToLower(asset.Digest), "sha256:")
			break
		}
	}
	if downloadURL == "" || len(expectedHash) != sha256.Size*2 {
		return "", "", errors.New("official release is missing the Windows CLI or its SHA-256 digest")
	}
	if VerifyFile(installerPath, expectedHash) == nil {
		_ = writeMetadata(metadataPath, cacheMetadata{Tag: latest.TagName, SHA256: expectedHash})
		return installerPath, expectedHash, nil
	}

	temporaryPath := installerPath + ".download"
	if err := d.download(ctx, downloadURL, temporaryPath); err != nil {
		return "", "", err
	}
	defer os.Remove(temporaryPath)
	if err := VerifyFile(temporaryPath, expectedHash); err != nil {
		return "", "", fmt.Errorf("verify official installer: %w", err)
	}
	backupPath := installerPath + ".previous"
	_ = os.Remove(backupPath)
	hadPrevious := false
	if _, err := os.Stat(installerPath); err == nil {
		if err := os.Rename(installerPath, backupPath); err != nil {
			return "", "", fmt.Errorf("preserve previous official CLI: %w", err)
		}
		hadPrevious = true
	}
	if err := os.Rename(temporaryPath, installerPath); err != nil {
		if hadPrevious {
			_ = os.Rename(backupPath, installerPath)
		}
		return "", "", fmt.Errorf("install official CLI: %w", err)
	}
	_ = os.Remove(backupPath)
	if err := writeMetadata(metadataPath, cacheMetadata{Tag: latest.TagName, SHA256: expectedHash}); err != nil {
		return "", "", err
	}
	return installerPath, expectedHash, nil
}

func (d *Downloader) fetchRelease(ctx context.Context) (release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "VencordGuard (https://github.com/itousouta15/VencordGuard)")
	response, err := d.Client.Do(request)
	if err != nil {
		return release{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub returned %s", response.Status)
	}
	var value release
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&value); err != nil {
		return release{}, err
	}
	return value, nil
}

func (d *Downloader) download(ctx context.Context, url, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "VencordGuard (https://github.com/itousouta15/VencordGuard)")
	response, err := d.Client.Do(request)
	if err != nil {
		return fmt.Errorf("download official installer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download official installer: %s", response.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, io.LimitReader(response.Body, 64*1024*1024))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func VerifyFile(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func writeMetadata(path string, value cacheMetadata) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
