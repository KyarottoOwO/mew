package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
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
			return nil, "", fmt.Errorf("failed to extract MediaFire download URL: %v", err)
		}
		url = directURL
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
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

func extractMediaFireFolderLinks(client *http.Client, folderURL string) ([]mediaFireFileEntry, error) {
	folderKey := ""
	parts := strings.Split(folderURL, "/")
	for i, p := range parts {
		if p == "folder" && i+1 < len(parts) {
			folderKey = parts[i+1]
			break
		}
	}
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
		Files []mfFile `json:"files"`
	}
	type mfResponse struct {
		FolderContent mfFolderContent `json:"folder_content"`
	}
	type mfAPIResponse struct {
		Response mfResponse `json:"response"`
	}

	var allFiles []mediaFireFileEntry
	chunk := 1
	for {
		apiURL := fmt.Sprintf("https://www.mediafire.com/api/1.5/folder/get_content.php?folder_key=%s&content_type=files&response_format=json&chunk=%d&first_item=1&last_item=100", folderKey, chunk)

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch MediaFire API: %v", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read API response: %v", err)
		}

		var apiResp mfAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("failed to parse API response: %v", err)
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

		if len(files) < 100 {
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
			return nil, "", fmt.Errorf("failed to get MediaFire download link: %v", err)
		}
		dlURL = extracted
	}

	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		return nil, "", fmt.Errorf("got HTML instead of file (likely a bad download link)")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return data, extractFilename(resp, dlURL), nil
}

func extractMediaFireDownloadURL(client *http.Client, pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch MediaFire page: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
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
