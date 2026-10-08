package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// subwayRedPackDir locates the SUBWAYRED resource pack. These tests exercise
// the real pack on disk, so they skip when it is not installed.
const subwayRedPack = "SUBWAYRED"

func findTestPack(t *testing.T, name string) (*App, string) {
	t.Helper()
	base := filepath.Join(os.Getenv("APPDATA"), "Minecraft Bedrock", "Users", "Shared", "games", "com.mojang", "resource_packs")
	dir := filepath.Join(base, name)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("pack %q not installed at %s", name, base)
	}
	a := NewApp(false)
	a.settings["resourcePacksPath"] = base
	return a, base
}

// jsonEntry finds a JSON resource by exact path.
func jsonEntry(t *testing.T, res []UiResource, path string) UiResource {
	t.Helper()
	for _, r := range res {
		if r.Path == path {
			return r
		}
	}
	t.Fatalf("resource %q not returned (got %d resources)", path, len(res))
	return UiResource{}
}

func TestSubwayRedHasUiScreens(t *testing.T) {
	a, _ := findTestPack(t, subwayRedPack)
	if !a.PackHasUiScreens(subwayRedPack) {
		t.Errorf("PackHasUiScreens(%q) = false, want true", subwayRedPack)
	}
}

func TestSubwayRedUiResourcesHaveContent(t *testing.T) {
	a, _ := findTestPack(t, subwayRedPack)
	res, err := a.GetPackAllUiResources(subwayRedPack)
	if err != nil {
		t.Fatalf("GetPackAllUiResources: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected resources")
	}

	// The four ui json files the pack ships must be returned with real content,
	// because the renderer cannot discover screens from paths alone.
	for _, want := range []string{
		"ui/_ui_defs.json",
		"ui/start_screen.json",
		"ui/startup_screen.json",
		"ui/ui_common_enhanced.json",
	} {
		e := jsonEntry(t, res, want)
		if e.IsBinary {
			t.Errorf("%s: expected text resource", want)
		}
		if len(e.Content) == 0 {
			t.Errorf("%s: empty content", want)
		}
	}

	// The pack's startup screen is the control that declares "type": "screen",
	// which is what PackHasUiScreens keys off. start_screen.json only overrides
	// children and inherits from vanilla bases.
	startup := jsonEntry(t, res, "ui/startup_screen.json")
	if !strings.Contains(startup.Content, `"type": "screen"`) &&
		!strings.Contains(startup.Content, `"type":"screen"`) {
		t.Errorf("startup_screen.json does not declare a screen control")
	}
	if !strings.Contains(startup.Content, `"namespace"`) {
		t.Errorf("startup_screen.json has no namespace")
	}
}

func TestSubwayRedUiDefsParseAndListScreen(t *testing.T) {
	a, _ := findTestPack(t, subwayRedPack)
	res, err := a.GetPackAllUiResources(subwayRedPack)
	if err != nil {
		t.Fatalf("GetPackAllUiResources: %v", err)
	}
	raw := jsonEntry(t, res, "ui/_ui_defs.json")

	// Bedrock allows // comments, so plain json.Unmarshal must fail here.
	// The renderer tolerates them; this confirms the file needs such tolerance.
	var strict struct {
		Defs []string `json:"ui_defs"`
	}
	strictErr := json.Unmarshal([]byte(raw.Content), &strict)

	var tolerant struct {
		Defs []string `json:"ui_defs"`
	}
	if err := json.Unmarshal(stripJSONComments([]byte(raw.Content)), &tolerant); err != nil {
		t.Fatalf("comment-stripped _ui_defs.json did not parse: %v", err)
	}
	if len(tolerant.Defs) == 0 {
		t.Fatal("ui_defs list is empty after stripping comments")
	}
	t.Logf("_ui_defs.json: %d defs, strict parse error = %v", len(tolerant.Defs), strictErr)

	if !containsString(tolerant.Defs, "ui/start_screen.json") {
		t.Errorf("ui_defs does not list start_screen.json")
	}
	if !containsString(tolerant.Defs, "ui/ui_common_enhanced.json") {
		t.Errorf("ui_defs does not list ui_common_enhanced.json")
	}
}

func TestSubwayRedTextureResourcesAreLazy(t *testing.T) {
	a, _ := findTestPack(t, subwayRedPack)
	res, err := a.GetPackAllUiResources(subwayRedPack)
	if err != nil {
		t.Fatalf("GetPackAllUiResources: %v", err)
	}

	var imgPaths []string
	for _, r := range res {
		if !r.IsBinary {
			continue
		}
		if !strings.HasPrefix(r.Path, "textures/") {
			t.Errorf("binary resource outside textures/: %s", r.Path)
		}
		imgPaths = append(imgPaths, r.Path)
	}
	if len(imgPaths) == 0 {
		t.Fatal("expected texture resources")
	}

	// textures/ui and textures/gui ship inline; the bulky block/item textures
	// must stay pending so a pack load does not transfer megabytes.
	inline, pending := 0, 0
	for _, r := range res {
		if !r.IsBinary {
			continue
		}
		if r.Content != "" {
			inline++
			if !strings.HasPrefix(r.Path, "textures/ui/") && !strings.HasPrefix(r.Path, "textures/gui/") {
				t.Errorf("unexpected inline bytes for %s", r.Path)
			}
		} else {
			pending++
		}
	}
	if inline == 0 {
		t.Error("expected textures/ui or textures/gui to be inlined")
	}
	if pending == 0 {
		t.Error("expected non-ui textures to be pending")
	}
	t.Logf("textures: %d total, %d inline, %d pending", len(imgPaths), inline, pending)
}

// TestSubwayRedFetchOnDemandTexture fetches a pending texture and checks the
// resolved path matches the real file, since the renderer matches extensionless
// json references against registered file paths.
func TestSubwayRedFetchOnDemandTexture(t *testing.T) {
	a, base := findTestPack(t, subwayRedPack)

	// Pick a texture that is registered as pending, then fetch its bytes.
	res, err := a.GetPackAllUiResources(subwayRedPack)
	if err != nil {
		t.Fatalf("GetPackAllUiResources: %v", err)
	}
	var pending string
	for _, r := range res {
		if r.IsBinary && r.Content == "" {
			pending = r.Path
			break
		}
	}
	if pending == "" {
		t.Skip("no pending texture available")
	}

	file, err := a.GetPackFile(subwayRedPack, pending)
	if err != nil {
		t.Fatalf("GetPackFile(%s): %v", pending, err)
	}
	if file.Content == "" {
		t.Fatalf("GetPackFile(%s) returned empty content", pending)
	}
	if file.Path != pending {
		t.Errorf("resolved path = %q, want %q", file.Path, pending)
	}
	if filepath.Ext(file.Path) == "" {
		t.Errorf("resolved path %q has no extension; the renderer needs one", file.Path)
	}

	// The same reference without an extension must resolve to the same file.
	if ext := filepath.Ext(pending); ext != "" {
		stem := strings.TrimSuffix(pending, ext)
		stemFile, err := a.GetPackFile(subwayRedPack, stem)
		if err != nil {
			t.Fatalf("GetPackFile(%s): %v", stem, err)
		}
		if stemFile.Path != pending {
			t.Errorf("extensionless lookup resolved to %q, want %q", stemFile.Path, pending)
		}
		if stemFile.Content != file.Content {
			t.Errorf("extensionless lookup returned different bytes")
		}
	}

	// Compare against the real file to prove the bytes match.
	raw, err := os.ReadFile(filepath.Join(base, subwayRedPack, filepath.FromSlash(file.Path)))
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("source texture is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		t.Fatalf("decode content: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Errorf("returned bytes (%d) differ from file on disk (%d)", len(decoded), len(raw))
	}
	t.Logf("fetched %s (%d bytes on disk)", file.Path, len(raw))
}

func TestSubwayRedFetchRejectsTraversal(t *testing.T) {
	a, _ := findTestPack(t, subwayRedPack)
	bad := []string{
		"../../../Windows/win.ini",
		"..\\..\\Windows\\win.ini",
		"/etc/passwd",
		"ui/../manifest.json",
		"behavior_packs/../ui/x.json",
		"",
		".",
	}
	for _, p := range bad {
		if f, err := a.GetPackFile(subwayRedPack, p); err == nil {
			t.Errorf("GetPackFile(%q) should have failed, returned %d bytes", p, len(f.Content))
		}
	}
}

func TestSubwayRedManifestParses(t *testing.T) {
	_, base := findTestPack(t, subwayRedPack)
	data, err := os.ReadFile(filepath.Join(base, subwayRedPack, "manifest.json"))
	if err != nil {
		t.Skipf("no manifest.json: %v", err)
	}
	var m struct {
		Header struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			UUID        string `json:"uuid"`
			// Bedrock manifests encode version as an array of ints.
			Version []int `json:"version"`
		} `json:"header"`
		MinEngineVersion []int `json:"min_engine_version"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("manifest.json invalid: %v", err)
	}
	if m.Header.Name == "" || m.Header.UUID == "" {
		t.Errorf("manifest header incomplete: %+v", m.Header)
	}
	t.Logf("pack %q uuid=%s version=%v", m.Header.Name, m.Header.UUID, m.Header.Version)
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
