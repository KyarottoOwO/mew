package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// stripJSONComments removes // comments so Bedrock's comment-tolerant json can
// be parsed by encoding/json. This covers both whole-line and trailing
// comments. A // sequence inside a string literal would be stripped too, which
// no vanilla ui definition relies on.
func stripJSONComments(data []byte) []byte {
	return commentLineRe.ReplaceAll(data, nil)
}

// Vanilla JSON-UI assets are fetched on demand from a public mirror of the
// Minecraft: Bedrock Edition vanilla resource pack and cached on disk. Only the
// files a pack actually needs are downloaded: the ui definition list, every ui
// json it names, and textures requested by the renderer while laying out a
// screen. Nothing vanilla is bundled with the app.
const (
	vanillaRawBase  = "https://raw.githubusercontent.com/bedrock-dot-dev/packs/master/stable/resource/"
	vanillaDefsPath = "ui/_ui_defs.json"
	vanillaProgress = "jsonuiVanillaProgress"
	vanillaTimeout  = 25 * time.Second
	vanillaWorkers  = 8
)

// Paths must stay inside the vanilla resource tree; nothing else is fetchable.
var (
	vanillaFetchRe = regexp.MustCompile(`^(ui|textures|texts)/[A-Za-z0-9_./@-]+$`)
	commentLineRe  = regexp.MustCompile(`//[^\r\n]*`)
)

var (
	vanillaClient = &http.Client{Timeout: vanillaTimeout}
)

type vanillaFetchState struct {
	mu         sync.Mutex
	inflight   map[string]*vanillaFetch
	succeeded  bool
	generation int
}

// vanillaFetch is one in-flight download. Callers that ask for the same file
// while it downloads wait on done instead of starting a second fetch, so every
// render that wants the texture gets it.
type vanillaFetch struct {
	done chan struct{}
	data []byte
	err  error
}

// errVanillaNotFound means the mirror answered 404: the file does not exist, as
// opposed to a transient network or server error. Only this is worth caching as
// missing.
var errVanillaNotFound = errors.New("vanilla file not found")

var vanillaState = &vanillaFetchState{inflight: map[string]*vanillaFetch{}}

func vanillaCacheDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "jsonui-vanilla")
}

// normalizeResourcePath cleans a caller supplied resource path and rejects
// anything outside the pack resource tree.
func normalizeResourcePath(filePath string) string {
	rel := filepath.ToSlash(strings.TrimSpace(filePath))
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return ""
	}
	if strings.HasPrefix(rel, "../") || strings.Contains(rel, "..") {
		return ""
	}
	return rel
}

// textureExts are the image formats Bedrock ships for ui textures.
var textureExts = []string{".png", ".tga", ".jpg", ".jpeg"}

// resolveCandidates returns rel itself plus each extension variant, so callers
// that pass an extensionless json reference still find the real file.
func resolveCandidates(rel string) []string {
	out := make([]string, 0, len(textureExts)+1)
	if strings.ToLower(filepath.Ext(rel)) != "" {
		out = append(out, rel)
		return out
	}
	for _, ext := range textureExts {
		out = append(out, rel+ext)
	}
	return out
}

// safePackFile joins rel under base and refuses to escape it.
func safePackFile(base, rel string) (string, error) {
	if base == "" || rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid path")
	}
	full := filepath.Join(base, filepath.FromSlash(rel))
	root := filepath.Clean(base)
	if !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path")
	}
	return full, nil
}

// vanillaCachePath maps a resource path to its on-disk location, rejecting
// anything that tries to escape the cache root.
func vanillaCachePath(rel string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(filepath.ToSlash(rel), "/"))
	if clean == "." || strings.HasPrefix(clean, "../") || !vanillaFetchRe.MatchString(clean) {
		return "", fmt.Errorf("invalid vanilla path")
	}
	full := filepath.Join(vanillaCacheDir(), filepath.FromSlash(clean))
	root := filepath.Clean(vanillaCacheDir())
	if full != root && !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid vanilla path")
	}
	return full, nil
}

func (a *App) emitVanillaProgress(done, total int) {
	if a.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(a.ctx, vanillaProgress, map[string]interface{}{
		"done":  done,
		"total": total,
	})
}

// downloadVanillaFile fetches rel straight to the cache without touching the
// inflight bookkeeping. Callers hold no locks.
func (a *App) downloadVanillaFile(rel, full string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(full), os.ModePerm); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, vanillaRawBase+rel, nil)
	if err != nil {
		return nil, err
	}
	resp, err := vanillaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", errVanillaNotFound, rel)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vanilla fetch failed (%s): HTTP %d", rel, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Best effort cache write; a failed write only costs another download.
	_ = os.WriteFile(full, data, 0644)

	vanillaState.mu.Lock()
	vanillaState.succeeded = true
	vanillaState.generation++
	vanillaState.mu.Unlock()
	return data, nil
}

// vanillaGeneration counts how many vanilla files have landed (and cache
// clears). Renders fold it into their cache key, so a render drawn while a
// texture was missing is rebuilt the moment that texture arrives instead of
// being cached as a permanent blank.
func vanillaGeneration() int {
	vanillaState.mu.Lock()
	defer vanillaState.mu.Unlock()
	return vanillaState.generation
}

// readVanillaFile returns cached bytes for rel, downloading them on first use.
// Downloads are de-duplicated so a screen asking for the same texture in
// several rounds only fetches it once.
func (a *App) readVanillaFile(rel string) ([]byte, error) {
	full, err := vanillaCachePath(rel)
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(full); err == nil && len(data) > 0 {
		return data, nil
	}

	vanillaState.mu.Lock()
	if f, ok := vanillaState.inflight[rel]; ok {
		vanillaState.mu.Unlock()
		// Another render is already fetching this file: wait for it instead of
		// failing. Returning early here is what let a texture two renders wanted
		// at once be cached as missing. A failure is handed to every waiter but
		// not cached, so a later render can retry.
		<-f.done
		return f.data, f.err
	}
	f := &vanillaFetch{done: make(chan struct{})}
	vanillaState.inflight[rel] = f
	vanillaState.mu.Unlock()

	f.data, f.err = a.downloadVanillaFile(rel, full)

	vanillaState.mu.Lock()
	delete(vanillaState.inflight, rel)
	vanillaState.mu.Unlock()
	close(f.done)
	return f.data, f.err
}

// rewriteUiDefs rewrites a _ui_defs.json payload so its ui_defs array contains
// exactly the entries the renderer can load: listed-and-available, plus any
// extra definitions we want registered. Order is preserved and extras are
// appended, keeping the generated list deterministic.
func rewriteUiDefs(raw []byte, available map[string]bool, extras []string) []byte {
	var defs struct {
		Defs []string `json:"ui_defs"`
	}
	if err := json.Unmarshal(stripJSONComments(raw), &defs); err != nil {
		return raw
	}

	seen := map[string]bool{}
	out := make([]string, 0, len(defs.Defs)+len(extras))
	for _, d := range defs.Defs {
		if seen[d] || !available[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}
	for _, e := range extras {
		if seen[e] || !available[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	defs.Defs = out

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(defs); err != nil {
		return raw
	}
	return buf.Bytes()
}

// Vanilla globals and the definition list itself are not covered by the list's
// own contents, but the renderer needs _ui_defs.json staged to register every
// vanilla ui file as a definition and _global_variables.json for $var lookups.
var vanillaExtraFiles = []string{
	"ui/_ui_defs.json",
	"ui/_global_variables.json",
	"texts/languages.json",
}

type VanillaUiFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// GetVanillaUiFiles returns the vanilla ui json files as text, ready to be
// staged as the base layer under a pack. It fetches the definition list once
// (cached) and then each file it names, a few at a time. Files that fail to
// download are skipped so a partial mirror never blocks rendering.
func (a *App) GetVanillaUiFiles() ([]VanillaUiFile, error) {
	defsData, err := a.readVanillaFile(vanillaDefsPath)
	if err != nil {
		return nil, fmt.Errorf("vanilla ui defs unavailable: %v", err)
	}

	var defs struct {
		Defs []string `json:"ui_defs"`
	}
	if err := json.Unmarshal(stripJSONComments(defsData), &defs); err != nil {
		return nil, fmt.Errorf("vanilla ui defs invalid: %v", err)
	}

	rels := make([]string, 0, len(defs.Defs)+len(vanillaExtraFiles))
	rels = append(rels, defs.Defs...)
	rels = append(rels, vanillaExtraFiles...)

	total := len(rels)
	results := make([]VanillaUiFile, total)
	var mu sync.Mutex
	var done int
	var wg sync.WaitGroup
	jobs := make(chan int)

	worker := func() {
		defer wg.Done()
		for i := range jobs {
			rel := rels[i]
			full, pathErr := vanillaCachePath(rel)
			if pathErr != nil {
				continue
			}
			data, readErr := os.ReadFile(full)
			if readErr != nil || len(data) == 0 {
				data, readErr = a.downloadVanillaFile(rel, full)
			}
			mu.Lock()
			if readErr != nil {
				a.logDebug(fmt.Sprintf("vanilla: skipping %s (%v)", rel, readErr))
			} else {
				results[i] = VanillaUiFile{Path: rel, Content: string(data)}
			}
			done++
			mu.Unlock()
			a.emitVanillaProgress(done, total)
		}
	}

	wg.Add(vanillaWorkers)
	for w := 0; w < vanillaWorkers; w++ {
		go worker()
	}
	for i := range rels {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	files := make([]VanillaUiFile, 0, total)
	fetched := map[string]bool{}
	for _, f := range results {
		if f.Path != "" {
			files = append(files, f)
			fetched[f.Path] = true
		}
	}

	// The mirror is missing a few files the upstream defs list still names
	// (editor_mode_screen.json, for one). Registering only what we actually
	// have keeps the renderer from warning about absent definitions.
	//
	// Nothing is appended: ui_defs may only list ui definitions, so adding
	// _ui_defs.json itself, _global_variables.json or texts/languages.json
	// makes the renderer try to read variables and localisation tables as
	// controls. Those files are still staged, which is what resolves $vars.
	for i := range files {
		if files[i].Path == vanillaDefsPath {
			files[i].Content = string(rewriteUiDefs([]byte(files[i].Content), fetched, nil))
		}
	}

	a.logDebug(fmt.Sprintf("vanilla: %d/%d ui files available", len(files), total))
	return files, nil
}

// GetVanillaFileBytes returns a single vanilla file as base64, for textures the
// renderer requests during layout. Pack-local files always win; this is only
// the fallback for assets the pack does not ship.
func (a *App) GetVanillaFileBytes(filePath string) (string, error) {
	rel := normalizeResourcePath(filePath)
	if rel == "" {
		return "", fmt.Errorf("invalid vanilla path")
	}
	data, err := a.readVanillaFile(rel)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// UiFile carries a texture back to the renderer. The renderer matches json
// references (which have no extension) against registered file paths, so the
// resolved path must include the real extension.
type UiFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// GetVanillaFile resolves a texture path and returns its bytes plus the real
// file path, trying the common image extensions when the caller passes none.
func (a *App) GetVanillaFile(filePath string) (UiFile, error) {
	rel := normalizeResourcePath(filePath)
	if rel == "" {
		return UiFile{}, fmt.Errorf("invalid vanilla path")
	}
	for _, candidate := range resolveCandidates(rel) {
		if data, err := a.readVanillaFile(candidate); err == nil {
			return UiFile{Path: candidate, Content: base64.StdEncoding.EncodeToString(data)}, nil
		}
	}
	return UiFile{}, fmt.Errorf("vanilla file not found: %s", rel)
}

// GetPackFile is GetVanillaFile for a specific pack, used before falling back to
// the vanilla base so a pack's own texture always wins.
func (a *App) GetPackFile(packName string, filePath string) (UiFile, error) {
	rel := normalizeResourcePath(filePath)
	if rel == "" {
		return UiFile{}, fmt.Errorf("invalid path")
	}
	for _, candidate := range resolveCandidates(rel) {
		if _, err := vanillaCachePath(candidate); err != nil {
			continue
		}
		full, err := safePackFile(a.getPackDir(packName), candidate)
		if err != nil {
			continue
		}
		if data, readErr := os.ReadFile(full); readErr == nil {
			return UiFile{Path: candidate, Content: base64.StdEncoding.EncodeToString(data)}, nil
		}
	}
	return UiFile{}, fmt.Errorf("pack file not found: %s", rel)
}

// ClearVanillaCache drops every cached vanilla file.
func (a *App) ClearVanillaCache() error {
	dir := vanillaCacheDir()
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	vanillaState.mu.Lock()
	vanillaState.succeeded = false
	vanillaState.generation++
	vanillaState.mu.Unlock()
	return nil
}

type VanillaCacheStatus struct {
	Cached    int   `json:"cached"`
	SizeKB    int64 `json:"sizeKb"`
	Ready     bool  `json:"ready"`
	Available bool  `json:"available"`
}

// GetVanillaCacheStatus reports how much is cached and whether the mirror
// answered at least once, so the UI can explain an offline renderer.
func (a *App) GetVanillaCacheStatus() VanillaCacheStatus {
	st := VanillaCacheStatus{}
	dir := vanillaCacheDir()
	hasDefs := false
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		st.Cached++
		st.SizeKB += info.Size() / 1024
		if strings.HasSuffix(filepath.ToSlash(p), vanillaDefsPath) {
			hasDefs = true
		}
		return nil
	})
	st.Ready = hasDefs
	vanillaState.mu.Lock()
	st.Available = vanillaState.succeeded
	vanillaState.mu.Unlock()
	return st
}
