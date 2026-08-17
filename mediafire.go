package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func downloadFromURL(url string) ([]byte, string, error) {
	client := createHTTPClient()

	if isMediaFireURL(url) {
		directURL, err := extractMediaFireDownloadURL(client, url)
		if err != nil {
			logError(formatError("extractMediaFireDownloadURL failed", err))
			return nil, "", fmt.Errorf("failed to extract MediaFire download URL: %v", err)
		}
		url = directURL
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logError(formatError("create request failed", err))
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		logError(formatError(fmt.Sprintf("download failed for %s", url), err))
		return nil, "", fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logError(fmt.Sprintf("download failed for %s: HTTP %d", url, resp.StatusCode))
		return nil, "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logError(formatError(fmt.Sprintf("read response failed for %s", url), err))
		return nil, "", fmt.Errorf("failed to read response body: %v", err)
	}

	filename := extractFilename(resp, url)

	return data, filename, nil
}

func isMediaFireURL(url string) bool {
	return strings.Contains(strings.ToLower(url), "mediafire.com")
}

func isMediaFireFolderURL(url string) bool {
	return isMediaFireURL(url) && strings.Contains(url, "/folder/")
}

type mediaFireFileEntry struct {
	Name string
	URL  string
}

func extractMediaFireFolderKey(folderURL string) string {
	parts := strings.Split(folderURL, "/")
	for i, p := range parts {
		if p == "folder" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

var mediaFireAPIBase = "https://www.mediafire.com/api/1.5"

func extractMediaFireFolderLinks(client *http.Client, folderURL string) ([]mediaFireFileEntry, error) {
	folderKey := extractMediaFireFolderKey(folderURL)
	if folderKey == "" {
		return nil, fmt.Errorf("could not extract folder key from URL")
	}

	type mfFileLink struct {
		NormalDownload string `json:"normal_download"`
	}
	type mfFile struct {
		Quickkey string     `json:"quickkey"`
		Filename string     `json:"filename"`
		Links    mfFileLink `json:"links"`
	}
	type mfFolderContent struct {
		Files      []mfFile `json:"files"`
		MoreChunks string   `json:"more_chunks"`
	}
	type mfResponse struct {
		FolderContent mfFolderContent `json:"folder_content"`
		Result        string          `json:"result"`
		Message       string          `json:"message"`
	}
	type mfAPIResponse struct {
		Response mfResponse `json:"response"`
	}

	var allFiles []mediaFireFileEntry
	chunk := 1
	for {
		apiURL := fmt.Sprintf("%s/folder/get_content.php?folder_key=%s&content_type=files&response_format=json&chunk=%d&first_item=1&last_item=100", mediaFireAPIBase, folderKey, chunk)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			logError(formatError(fmt.Sprintf("MediaFire API request failed (key=%s, chunk=%d)", folderKey, chunk), err))
			return nil, fmt.Errorf("failed to fetch MediaFire API: %v", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logError(formatError("read MediaFire API response failed", err))
			return nil, fmt.Errorf("failed to read API response: %v", err)
		}

		var apiResp mfAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			logError(formatError("parse MediaFire API JSON failed", err))
			return nil, fmt.Errorf("failed to parse API response: %v", err)
		}

		if !strings.EqualFold(apiResp.Response.Result, "Success") {
			msg := apiResp.Response.Message
			if msg == "" {
				msg = "MediaFire API returned an error"
			}
			logError(fmt.Sprintf("MediaFire API error (key=%s): %s", folderKey, msg))
			return nil, fmt.Errorf("MediaFire API error: %s", msg)
		}

		files := apiResp.Response.FolderContent.Files
		if len(files) == 0 {
			break
		}

		for _, f := range files {
			if f.Links.NormalDownload == "" {
				continue
			}
			name := f.Filename
			name = strings.ReplaceAll(name, "+", " ")
			allFiles = append(allFiles, mediaFireFileEntry{
				Name: name,
				URL:  f.Links.NormalDownload,
			})
		}

		if len(files) < 100 || strings.EqualFold(apiResp.Response.FolderContent.MoreChunks, "no") {
			break
		}
		chunk++
	}

	return allFiles, nil
}

func downloadDirect(client *http.Client, url string) ([]byte, string, error) {
	dlURL := url
	if isMediaFireURL(url) && !isMediaFireFolderURL(url) {
		extracted, err := extractMediaFireDownloadURL(client, url)
		if err != nil {
			logError(formatError(fmt.Sprintf("extractMediaFireDownloadURL failed for %s", url), err))
			return nil, "", fmt.Errorf("failed to get MediaFire download link: %v", err)
		}
		dlURL = extracted
	}

	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		logError(formatError("create download request failed", err))
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		logError(formatError(fmt.Sprintf("download failed for %s", dlURL), err))
		return nil, "", fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logError(fmt.Sprintf("download failed for %s: HTTP %d", dlURL, resp.StatusCode))
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		logError(fmt.Sprintf("download failed for %s: got HTML (Content-Type: %s)", dlURL, ct))
		return nil, "", fmt.Errorf("got HTML instead of file (likely a bad download link)")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logError(formatError(fmt.Sprintf("read download body failed for %s", dlURL), err))
		return nil, "", err
	}

	return data, extractFilename(resp, dlURL), nil
}

func resolveDownloadURL(base *url.URL, loc string) string {
	if strings.HasPrefix(loc, "//") {
		return "https:" + loc
	}
	ref, err := url.Parse(loc)
	if err != nil {
		return loc
	}
	return base.ResolveReference(ref).String()
}

func extractMediaFireDownloadURL(client *http.Client, pageURL string) (string, error) {
	noRedirect := &http.Client{
		Timeout:   client.Timeout,
		Transport: client.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := noRedirect.Do(req)
	if err != nil {
		logError(formatError(fmt.Sprintf("fetch MediaFire page failed: %s", pageURL), err))
		return "", fmt.Errorf("failed to fetch MediaFire page: %v", err)
	}
	defer resp.Body.Close()

	// MediaFire currently redirects file pages straight to the download
	// server, so the Location header is already the direct download link.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			return resolveDownloadURL(resp.Request.URL, loc), nil
		}
	}

	if resp.StatusCode != http.StatusOK {
		logError(fmt.Sprintf("MediaFire page returned status %d for %s", resp.StatusCode, pageURL))
		return "", fmt.Errorf("unexpected status %d from MediaFire", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		logError(formatError(fmt.Sprintf("read MediaFire page failed: %s", pageURL), err))
		return "", fmt.Errorf("failed to read MediaFire page: %v", err)
	}

	html := string(body)

	patterns := []string{
		`id="downloadButton"[^>]*href="([^"]+)"`,
		`class="input popsok"[^>]*href="([^"]+)"`,
		`href="(https?://download\d*[^"]*\.zip[^"]*)"`,
		`href="(https?://[^"]*\.mediafire\.com/[^"]*)"`,
		`var\s+LimiServer\s*=\s*"([^"]+)"`,
		`class="download_link[^"]*"[^>]*href="([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			dlURL := strings.Trim(matches[1], `"`)
			if strings.HasPrefix(dlURL, "//") {
				dlURL = "https:" + dlURL
			}
			return dlURL, nil
		}
	}

	dlRe := regexp.MustCompile(`href="(https?://[^"]*)"`)
	allLinks := dlRe.FindAllStringSubmatch(html, -1)
	for _, match := range allLinks {
		if len(match) > 1 {
			link := match[1]
			if strings.Contains(link, "download") || strings.Contains(link, ".zip") {
				return link, nil
			}
		}
	}

	logError(fmt.Sprintf("could not find download link on MediaFire page: %s", pageURL))
	return "", fmt.Errorf("could not find download link on MediaFire page")
}

func extractFilename(resp *http.Response, url string) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		re := regexp.MustCompile(`filename[*]?="?([^";\n]+)"?`)
		matches := re.FindStringSubmatch(cd)
		if len(matches) > 1 {
			name := strings.TrimSpace(matches[1])
			name = strings.Trim(name, `"`)
			if name != "" {
				return name
			}
		}
	}

	parts := strings.Split(url, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && strings.Contains(parts[i], ".") {
			return parts[i]
		}
	}

	return "download.zip"
}
