package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestExtractMediaFireFolderKey(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.mediafire.com/folder/yzvdjsqmuvtem/versqa's_4k_pack_folder", "yzvdjsqmuvtem"},
		{"https://www.mediafire.com/folder/abc123", "abc123"},
		{"https://www.mediafire.com/file/quickkey/name.zip/file", ""},
		{"https://www.mediafire.com/folder", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractMediaFireFolderKey(c.url); got != c.want {
			t.Errorf("extractMediaFireFolderKey(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestResolveDownloadURL(t *testing.T) {
	base, _ := url.Parse("https://www.mediafire.com/file/quickkey/name.zip/file")

	cases := []struct {
		loc  string
		want string
	}{
		{"//download2261.mediafire.com/x/name.zip", "https://download2261.mediafire.com/x/name.zip"},
		{"https://download2261.mediafire.com/x/name.zip", "https://download2261.mediafire.com/x/name.zip"},
		{"/folder/xyz", "https://www.mediafire.com/folder/xyz"},
	}
	for _, c := range cases {
		if got := resolveDownloadURL(base, c.loc); got != c.want {
			t.Errorf("resolveDownloadURL(%q) = %q, want %q", c.loc, got, c.want)
		}
	}
}

func TestExtractMediaFireDownloadURLRedirect(t *testing.T) {
	direct := "https://download2261.mediafire.com/sig-token/name.zip"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", direct)
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	got, err := extractMediaFireDownloadURL(createHTTPClient(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != direct {
		t.Errorf("got %q, want %q", got, direct)
	}
}

func TestExtractMediaFireDownloadURLHtmlPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		fmt.Fprint(w, `<html><body><a id="downloadButton" href="https://download2261.mediafire.com/x/name.zip">Download</a></body></html>`)
	}))
	defer srv.Close()

	got, err := extractMediaFireDownloadURL(createHTTPClient(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://download2261.mediafire.com/x/name.zip" {
		t.Errorf("got %q, want download link", got)
	}
}

func TestExtractMediaFireDownloadURLNoLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		fmt.Fprint(w, `<html><body>Challenge page, no download link here.</body></html>`)
	}))
	defer srv.Close()

	_, err := extractMediaFireDownloadURL(createHTTPClient(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "could not find download link") {
		t.Errorf("expected missing-link error, got %v", err)
	}
}

func TestExtractMediaFireDownloadURLBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := extractMediaFireDownloadURL(createHTTPClient(), srv.URL); err == nil {
		t.Error("expected error for 403 response")
	}
}

func TestExtractFilename(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Disposition": []string{`attachment; filename="! mortal wound.zip"`}},
	}
	if got := extractFilename(resp, "https://download2261.mediafire.com/x/some.zip"); got != "! mortal wound.zip" {
		t.Errorf("extractFilename from header = %q", got)
	}

	empty := &http.Response{Header: http.Header{}}
	if got := extractFilename(empty, "https://download2261.mediafire.com/x/fallback.zip"); got != "fallback.zip" {
		t.Errorf("extractFilename from URL = %q", got)
	}
}

func TestExtractMediaFireFolderLinksErrorResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"response":{"action":"folder/get_content","message":"Unknown or invalid FolderKey","error":112,"result":"Error","current_api_version":"1.5"}}`)
	}))
	defer srv.Close()

	oldBase := mediaFireAPIBase
	mediaFireAPIBase = srv.URL
	defer func() { mediaFireAPIBase = oldBase }()

	_, err := extractMediaFireFolderLinks(createHTTPClient(), "https://www.mediafire.com/folder/xyz123")
	if err == nil || !strings.Contains(err.Error(), "Unknown or invalid FolderKey") {
		t.Errorf("expected API error message, got %v", err)
	}
}

func TestDownloadDirectMediaFireRedirect(t *testing.T) {
	fileBody := []byte("PK\x03\x04zip-content")
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/file_premium/") {
			w.Header().Set("Location", srv.URL+"/download/name.zip")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="name.zip"`)
		w.Write(fileBody)
	}))
	defer srv.Close()

	srvAddr := srv.Listener.Addr().String()
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = true
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srvAddr)
	}
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	data, name, err := downloadDirect(client, "https://www.mediafire.com/file_premium/qk/name.zip/file")
	if err != nil {
		t.Fatalf("downloadDirect failed: %v", err)
	}
	if string(data) != string(fileBody) {
		t.Errorf("data mismatch: got %d bytes", len(data))
	}
	if name != "name.zip" {
		t.Errorf("name = %q, want name.zip", name)
	}
}
