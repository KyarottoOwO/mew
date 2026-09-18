package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var currentVersion = "1.1.1"

const githubReleasesURL = "https://api.github.com/repos/KyarottoOwO/mew/releases/latest"

type UpdateInfo struct {
	NeedsUpdate   bool   `json:"needsUpdate"`
	LatestVersion string `json:"latestVersion"`
	DownloadURL   string `json:"downloadUrl"`
}

type ChangelogEntry struct {
	Tag  string `json:"tag"`
	URL  string `json:"url"`
	Date string `json:"date"`
	Body string `json:"body"`
}

func (a *App) GetVersion() string {
	return currentVersion
}

func (a *App) CheckForUpdate() UpdateInfo {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return UpdateInfo{NeedsUpdate: false}
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == "" || latestVersion == currentVersion {
		return UpdateInfo{NeedsUpdate: false}
	}

	downloadURL := ""
	for _, asset := range release.Assets {
		if strings.HasSuffix(strings.ToLower(asset.Name), ".exe") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return UpdateInfo{
		NeedsUpdate:   true,
		LatestVersion: latestVersion,
		DownloadURL:   downloadURL,
	}
}

// userDownloadsDir returns the user's Downloads folder when it exists, falling
// back to the home directory.
func userDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if env := os.Getenv("USERPROFILE"); env != "" {
			home = env
		}
	}
	if home == "" {
		home, _ = os.UserCacheDir()
	}
	downloads := filepath.Join(home, "Downloads")
	if st, err := os.Stat(downloads); err == nil && st.IsDir() {
		return downloads
	}
	return home
}

const updateProgressEvent = "updateProgress"

func (a *App) emitUpdateProgress(downloaded, total int64) {
	wailsRuntime.EventsEmit(a.ctx, updateProgressEvent, map[string]interface{}{
		"downloaded": downloaded,
		"total":      total,
	})
}

type updateProgressWriter struct {
	app        *App
	total      int64
	downloaded int64
	last       time.Time
}

func (w *updateProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.downloaded += int64(n)
	if time.Since(w.last) > 150*time.Millisecond {
		w.last = time.Now()
		w.app.emitUpdateProgress(w.downloaded, w.total)
	}
	return n, nil
}

// DownloadUpdate downloads the newest MEW installer into the Downloads folder
// and returns the saved file path. Progress is reported through the
// "updateProgress" event, throttled during the transfer.
func (a *App) DownloadUpdate(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("no download URL provided")
	}

	resp, err := http.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed (HTTP %d)", resp.StatusCode)
	}

	dir := userDownloadsDir()
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to access downloads folder: %v", err)
	}

	fileName := "mew.exe"
	tmpPath := filepath.Join(dir, fileName+".part")
	finalPath := filepath.Join(dir, fileName)

	// Make sure we end up with a plain "mew.exe", overwriting any earlier copy.
	_ = os.Remove(tmpPath)
	_ = os.Remove(finalPath)

	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}

	total := resp.ContentLength
	a.emitUpdateProgress(0, total)
	writer := &updateProgressWriter{app: a, total: total}
	_, copyErr := io.Copy(out, io.TeeReader(resp.Body, writer))
	closeErr := out.Close()

	if copyErr != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("download failed: %v", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("download failed: %v", closeErr)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to save update: %v", err)
	}

	a.emitUpdateProgress(total, total)
	a.logDebug(fmt.Sprintf("DownloadUpdate: saved %s", finalPath))
	return finalPath, nil
}

func (a *App) GetChangelog() []ChangelogEntry {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/KyarottoOwO/mew/releases", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var releases []struct {
		TagName   string `json:"tag_name"`
		HTMLURL   string `json:"html_url"`
		Published string `json:"published_at"`
		Body      string `json:"body"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil
	}

	var entries []ChangelogEntry
	for _, r := range releases {
		date := ""
		if t, err := time.Parse(time.RFC3339, r.Published); err == nil {
			date = t.Format("Jan 02, 2006")
		}
		entries = append(entries, ChangelogEntry{
			Tag:  strings.TrimPrefix(r.TagName, "v"),
			URL:  r.HTMLURL,
			Date: date,
			Body: r.Body,
		})
	}
	return entries
}

func (a *App) OpenFolder(dir string) {
	stat, err := os.Stat(dir)
	if err == nil && !stat.IsDir() {
		// Opening a file directly would launch it (or worse, run an .exe).
		// Reveal it inside its folder instead.
		switch runtime.GOOS {
		case "windows":
			exec.Command("explorer", "/select,\""+dir+"\"").Start()
			return
		case "darwin":
			exec.Command("open", "-R", dir).Start()
			return
		default:
			dir = filepath.Dir(dir)
		}
	}
	switch runtime.GOOS {
	case "windows":
		exec.Command("explorer", dir).Start()
	case "darwin":
		exec.Command("open", dir).Start()
	default:
		exec.Command("xdg-open", dir).Start()
	}
}

func (a *App) openInBrowser(rawURL string) {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return
	}
	wailsRuntime.BrowserOpenURL(a.ctx, rawURL)
}

func (a *App) OpenDiscordLink() {
	a.openInBrowser("https://discord.gg/nv9GrqTVM3")
}

func (a *App) OpenMewDataDir() {
	a.OpenFolder(filepath.Join(os.Getenv("LOCALAPPDATA"), "mew"))
}

func (a *App) OpenDonationLink() {
	a.openInBrowser("https://www.paypal.me/spotters1")
}

func (a *App) OpenDownloadLink(url string) {
	a.openInBrowser(url)
}

// InstallUpdate launches the downloaded installer so the update can proceed.
func (a *App) InstallUpdate(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("no update file path provided")
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return fmt.Errorf("update file not found: %s", path)
	}
	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start installer: %v", err)
	}
	return nil
}
