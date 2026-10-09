package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fakeResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

// withFakeVanilla points the vanilla client at a fake transport and the cache
// at a temp dir, restoring both afterwards.
func withFakeVanilla(t *testing.T, rt roundTripFunc) {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	prev := vanillaClient.Transport
	vanillaClient.Transport = rt
	t.Cleanup(func() { vanillaClient.Transport = prev })
}

// TestVanillaFetchCoalesces is the regression test for the item that vanished
// when two renders asked for the same not-yet-cached texture at once: one
// download must serve every caller, and no caller may fail.
func TestVanillaFetchCoalesces(t *testing.T) {
	var hits int32
	withFakeVanilla(t, func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&hits, 1)
		time.Sleep(50 * time.Millisecond) // hold so the others pile up
		return fakeResponse(http.StatusOK, []byte("png-bytes")), nil
	})

	a := &App{}
	const rel = "textures/items/diamond_hoe.png"
	const n = 12
	var wg sync.WaitGroup
	got := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data, err := a.readVanillaFile(rel)
			got[i], errs[i] = string(data), err
		}(i)
	}
	wg.Wait()

	if h := atomic.LoadInt32(&hits); h != 1 {
		t.Fatalf("expected one download for concurrent callers, got %d", h)
	}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("caller %d failed: %v", i, errs[i])
		}
		if got[i] != "png-bytes" {
			t.Fatalf("caller %d got %q", i, got[i])
		}
	}
}

// TestVanillaTransientErrorNotCached: a failed fetch must not be remembered as
// missing, or the texture stays gone for the rest of the session.
func TestVanillaTransientErrorNotCached(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	img := buf.Bytes()

	var hits int32
	withFakeVanilla(t, func(*http.Request) (*http.Response, error) {
		if atomic.AddInt32(&hits, 1) == 1 {
			return fakeResponse(http.StatusInternalServerError, nil), nil
		}
		return fakeResponse(http.StatusOK, img), nil
	})

	a := &App{}
	const rel = "textures/items/iron_hoe.png"
	if got := a.loadVanillaTexture(rel); got != nil {
		t.Fatalf("expected first (failing) load to return nil, got %T", got)
	}
	if got := a.loadVanillaTexture(rel); got == nil {
		t.Fatal("transient failure was cached as missing; retry returned nil")
	}
	if h := atomic.LoadInt32(&hits); h != 2 {
		t.Fatalf("expected a retry after the transient failure, got %d requests", h)
	}
}

// TestVanillaNotFoundCached: a real 404 is definitive and is cached, so the
// mirror is not asked again for a texture it does not have.
func TestVanillaNotFoundCached(t *testing.T) {
	withFakeVanilla(t, func(*http.Request) (*http.Response, error) {
		return fakeResponse(http.StatusNotFound, nil), nil
	})

	a := &App{}
	const rel = "textures/items/absent.png"
	if got := a.loadVanillaTexture(rel); got != nil {
		t.Fatalf("expected 404 to return nil, got %T", got)
	}
	if v, ok := a.thumbCache.Load("vtex\x00" + rel); !ok {
		t.Fatal("404 was not cached as missing")
	} else if _, missing := v.(missingTexture); !missing {
		t.Fatalf("cached value is %T, not missingTexture", v)
	}
}

// TestVanillaRenderAssets guards that the prefetch list stays derived from the
// renderer's allow-lists.
func TestVanillaRenderAssets(t *testing.T) {
	assets := vanillaRenderAssets()
	seen := map[string]bool{}
	for _, a := range assets {
		if seen[a] {
			t.Fatalf("duplicate prefetch asset %s", a)
		}
		seen[a] = true
	}
	for name := range vanillaItemNames {
		if !seen["textures/items/"+name+".png"] {
			t.Errorf("missing prefetch for item %s", name)
		}
	}
	for material := range vanillaArmorMaterials {
		for _, layer := range []int{1, 2} {
			want := fmt.Sprintf("textures/models/armor/%s_%d.png", material, layer)
			if !seen[want] {
				t.Errorf("missing prefetch for %s", want)
			}
		}
	}
	if !seen["textures/models/armor/elytra.png"] {
		t.Error("missing prefetch for elytra")
	}
}
