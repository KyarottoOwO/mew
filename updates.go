package main

import (
	"encoding/json"
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
	NeedsUpdate    bool   `json:"needsUpdate"`
	LatestVersion  string `json:"latestVersion"`
	DownloadURL    string `json:"downloadUrl"`
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
