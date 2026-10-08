package main

import (
	"encoding/json"
	"testing"
)

func TestVanillaUiFiles(t *testing.T) {
	a := &App{}
	files, err := a.GetVanillaUiFiles()
	if err != nil {
		t.Fatalf("GetVanillaUiFiles: %v", err)
	}
	t.Logf("vanilla ui files: %d", len(files))
	if len(files) < 100 {
		t.Fatalf("expected >100 vanilla ui files, got %d", len(files))
	}
	found := map[string]bool{}
	for _, f := range files {
		found[f.Path] = true
	}
	for _, want := range []string{"ui/ui_common.json", "ui/start_screen.json", "ui/pause_screen.json"} {
		if !found[want] {
			t.Errorf("missing expected vanilla file %s", want)
		}
	}
	for _, f := range files {
		if f.Path == "ui/ui_common.json" {
			if !containsStr(f.Content, "base_screen") {
				t.Errorf("ui_common.json missing base_screen")
			}
			if len(f.Content) < 1000 {
				t.Errorf("ui_common.json suspiciously small: %d bytes", len(f.Content))
			}
		}
	}
}

func TestVanillaFileBytesTexture(t *testing.T) {
	a := &App{}
	b64, err := a.GetVanillaFileBytes("textures/gui/gui.png")
	if err != nil {
		t.Fatalf("GetVanillaFileBytes: %v", err)
	}
	if len(b64) < 100 {
		t.Errorf("gui.png base64 too short: %d", len(b64))
	}
}

func TestVanillaRejectsEscape(t *testing.T) {
	a := &App{}
	if _, err := a.GetVanillaFileBytes("../../../etc/passwd"); err == nil {
		t.Errorf("expected traversal path to be rejected")
	}
	if _, err := a.GetVanillaFileBytes("behavior_packs/foo.json"); err == nil {
		t.Errorf("expected non-ui path to be rejected")
	}
}

func TestVanillaCacheStatus(t *testing.T) {
	a := &App{}
	st := a.GetVanillaCacheStatus()
	t.Logf("status: cached=%d sizeKb=%d ready=%v available=%v", st.Cached, st.SizeKB, st.Ready, st.Available)
}

func TestStripJSONComments(t *testing.T) {
	// The trailing-comment form is what real packs ship (verified against
	// SUBWAYRED's ui/_ui_defs.json) and it previously survived stripping,
	// leaving invalid json behind.
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"whole line", "{\n// note\n\"a\": 1\n}", []string{"a"}},
		{"trailing", "{\n\"a\": 1, // note\n\"b\": 2\n}", []string{"a", "b"}},
		{"trailing no comma", "{\n\"a\": 1 // note\n}", []string{"a"}},
		{"indented line", "{\n  // note\n  \"a\": 1\n}", []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out map[string]interface{}
			if err := json.Unmarshal(stripJSONComments([]byte(tc.in)), &out); err != nil {
				t.Fatalf("stripped json invalid: %v (input %q)", err, tc.in)
			}
			for _, k := range tc.want {
				if _, ok := out[k]; !ok {
					t.Errorf("key %q missing after strip: %v", k, out)
				}
			}
		})
	}
}

func containsStr(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
