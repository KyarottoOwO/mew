package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bedrockskin "github.com/THEBOSS9345/bedrock-skin-go"
)

// solidPNG encodes a solid-color PNG, the procedural texture the render tests
// use instead of shipping game art.
func solidPNG(t *testing.T, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// testApp builds an app rooted at a temp resource-packs dir with a temp
// vanilla cache, so nothing here touches the network.
func testApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	root := t.TempDir()
	a := NewApp(false)
	a.settings["resourcePacksPath"] = root
	// Tests never touch the network: vanilla textures come from the seeded
	// cache, and anything else fails fast.
	offlineVanilla(t)
	return a
}

func decodeDataURI(t *testing.T, uri string) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(dataURIBytes(t, uri)))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func dataURIBytes(t *testing.T, uri string) []byte {
	t.Helper()
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		t.Fatalf("not a data URI: %q", uri)
	}
	data, err := base64.StdEncoding.DecodeString(uri[comma+1:])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func countOpaque(img image.Image) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				n++
			}
		}
	}
	return n
}

func TestResolversPreferPackThenVanilla(t *testing.T) {
	a := testApp(t)
	root := a.getResourcePacksPath()

	packArmor := solidPNG(t, 64, 32, color.NRGBA{255, 0, 0, 255})
	vanillaArmor := solidPNG(t, 64, 32, color.NRGBA{0, 255, 0, 255})
	packSword := solidPNG(t, 16, 16, color.NRGBA{0, 0, 255, 255})
	vanillaSword := solidPNG(t, 16, 16, color.NRGBA{255, 255, 0, 255})

	writeFile(t, filepath.Join(root, "pack", "textures", "models", "armor", "diamond_1.png"), packArmor)
	writeFile(t, filepath.Join(vanillaCacheDir(), "textures", "models", "armor", "diamond_2.png"), vanillaArmor)
	writeFile(t, filepath.Join(root, "pack", "textures", "items", "diamond_sword.png"), packSword)
	writeFile(t, filepath.Join(vanillaCacheDir(), "textures", "items", "iron_sword.png"), vanillaSword)

	if got := a.armorLayer("pack", "diamond", 1); got == nil || !isRed(got) {
		t.Errorf("layer 1 should come from the pack")
	}
	if got := a.armorLayer("pack", "diamond", 2); got == nil || !isGreen(got) {
		t.Errorf("layer 2 should fall back to vanilla")
	}
	if got := a.itemTexture("pack", "diamond_sword"); got == nil || !isBlue(got) {
		t.Errorf("diamond_sword should come from the pack")
	}
	if got := a.itemTexture("pack", "iron_sword"); got == nil || !isYellow(got) {
		t.Errorf("iron_sword should fall back to vanilla")
	}
	if got := a.itemTexture("pack", "diamond_hoe"); got != nil {
		t.Errorf("an allow-listed but uncached item should be nil offline, got %v", got)
	}
}

// offlineVanilla points the vanilla client at a counting transport and restores
// it when the test ends.
func offlineVanilla(t *testing.T) *int {
	t.Helper()
	count := new(int)
	prev := vanillaClient.Transport
	vanillaClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		*count++
		return nil, errors.New("offline")
	})
	t.Cleanup(func() { vanillaClient.Transport = prev })
	return count
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestItemTextureUnknownNameNeverFetches(t *testing.T) {
	a := testApp(t)
	requests := offlineVanilla(t)

	if got := a.itemTexture("pack", "totally_made_up"); got != nil {
		t.Errorf("unknown item should be nil, got %v", got)
	}
	if got := a.itemTexture("pack", "../../secret"); got != nil {
		t.Errorf("path-like item should be nil, got %v", got)
	}
	if *requests != 0 {
		t.Errorf("unknown names must not hit the network, got %d requests", *requests)
	}
}

func TestRenderSkinBasics(t *testing.T) {
	a := testApp(t)
	root := a.getResourcePacksPath()
	skin := solidPNG(t, 64, 64, color.NRGBA{120, 120, 120, 255})
	writeFile(t, filepath.Join(root, "withSkin", "textures", "entity", "steve.png"), skin)
	writeFile(t, filepath.Join(root, "armorOnly", "textures", "models", "armor", "diamond_1.png"), skin)
	writeFile(t, filepath.Join(root, "armorOnly", "textures", "models", "armor", "diamond_2.png"), skin)

	for _, req := range []RenderRequest{
		{Pack: "withSkin", Size: 64},
		{Pack: "armorOnly", Material: "diamond", Size: 64},
		{Size: 64},
	} {
		uri, err := a.RenderSkin(req)
		if err != nil {
			t.Fatalf("%+v: %v", req, err)
		}
		if !strings.HasPrefix(uri, "data:image/png;base64,") {
			t.Fatalf("%+v: not a PNG data URI: %q", req, uri)
		}
		if got := decodeDataURI(t, uri).Bounds().Dx(); got != 64 {
			t.Fatalf("%+v: size %d, want 64", req, got)
		}
	}
}

func TestElytraReplacesChestplate(t *testing.T) {
	// The library drops the chestplate under an elytra; the service relies on
	// it, so pin the contract down.
	skin := onePxImage(color.NRGBA{120, 120, 120, 255})
	layer1 := onePxImage(color.NRGBA{200, 0, 0, 255})
	layer2 := onePxImage(color.NRGBA{0, 200, 0, 255})
	elytra := onePxImage(color.NRGBA{0, 0, 200, 255})

	both := bedrockskin.Options{Texture: skin, Size: 64}
	both.Armor = bedrockskin.ArmorSet(layer1, layer2)
	both.Armor.Elytra = elytra

	alone := bedrockskin.Options{Texture: skin, Size: 64}
	alone.Armor = bedrockskin.ArmorSet(layer1, layer2)
	alone.Armor.Chestplate = nil
	alone.Armor.Elytra = elytra

	a, err := both.RenderPNG()
	if err != nil {
		t.Fatal(err)
	}
	b, err := alone.RenderPNG()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("chestplate+elytra should render the same as the elytra alone")
	}
}

func onePxImage(c color.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, c)
	return img
}

func TestHideSkinWithoutEquipmentIsEmpty(t *testing.T) {
	a := testApp(t)
	_, err := a.RenderSkin(RenderRequest{HideSkin: true, Size: 64})
	if !errors.Is(err, bedrockskin.ErrEmptyView) {
		t.Fatalf("want ErrEmptyView, got %v", err)
	}
}

func TestRenderSkinFrames(t *testing.T) {
	a := testApp(t)
	frames, err := a.RenderSkinFrames(RenderRequest{Animation: "walk", FPS: 10, Frames: 4, Size: 64})
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 4 {
		t.Fatalf("got %d frames, want 4", len(frames))
	}
	if frames[0] == frames[1] {
		t.Error("walk frames should differ")
	}

	if _, err := a.RenderSkinFrames(RenderRequest{Animation: "not.an.animation", Size: 64}); err == nil {
		t.Error("expected an error for an unknown animation")
	}
	if _, err := a.RenderSkinFrames(RenderRequest{Size: 64}); err == nil {
		t.Error("expected an error when no animation is asked for")
	}

	names := a.ListAnimations()
	hasWalk, hasDance := false, false
	for _, n := range names {
		if n == "walk" {
			hasWalk = true
		}
		if n == "animation.player.dance" {
			hasDance = true
		}
	}
	if !hasWalk || !hasDance {
		t.Errorf("ListAnimations should include walk and the examples, got %v", names)
	}
}

func TestCameraMarginChangesSize(t *testing.T) {
	a := testApp(t)
	near, err := a.RenderSkin(RenderRequest{Size: 128, Camera: &CameraRequest{Margin: 0.6}})
	if err != nil {
		t.Fatal(err)
	}
	far, err := a.RenderSkin(RenderRequest{Size: 128, Camera: &CameraRequest{Margin: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if countOpaque(decodeDataURI(t, near)) <= countOpaque(decodeDataURI(t, far)) {
		t.Error("a smaller margin should draw the figure larger")
	}
}

func TestRenderRequestValidation(t *testing.T) {
	a := testApp(t)
	if _, err := a.renderOptions(RenderRequest{Pack: "../escape"}); err == nil {
		t.Error("expected a pack name with a path to be rejected")
	}
	if _, err := a.renderOptions(RenderRequest{Right: HandRequest{Item: "../../a"}}); err == nil {
		t.Error("expected an item name with a path to be rejected")
	}
	if _, err := a.renderOptions(RenderRequest{Parts: map[string]float64{"tail": 2}}); err == nil {
		t.Error("expected an unknown body part to be rejected")
	}
	if _, err := a.renderOptions(RenderRequest{Material: "topaz"}); err == nil {
		t.Error("expected an unknown armor material to be rejected")
	}

	opts, err := a.renderOptions(RenderRequest{Size: 5000, Camera: &CameraRequest{Pitch: 200, FOV: 400, Margin: 99}})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Size != 1024 {
		t.Errorf("size 5000 should clamp to 1024, got %d", opts.Size)
	}
	if opts.Camera.Pitch != 89 {
		t.Errorf("pitch 200 should clamp to 89, got %v", opts.Camera.Pitch)
	}
	if opts.Camera.FOV != 90 {
		t.Errorf("fov 400 should clamp to 90, got %v", opts.Camera.FOV)
	}
	if opts.Camera.Margin != 4 {
		t.Errorf("margin 99 should clamp to 4, got %v", opts.Camera.Margin)
	}
}

func TestIsHandEquipped(t *testing.T) {
	upright := []string{"diamond_sword", "wooden_axe", "netherite_pickaxe", "iron_shovel", "golden_hoe", "stick", "bone", "blaze_rod", "breeze_rod", "fishing_rod", "carrot_on_a_stick", "warped_fungus_on_a_stick", "mace"}
	flat := []string{"bread", "apple", "golden_apple", "cooked_beef", "ender_pearl", "bow", "totem"}
	for _, name := range upright {
		if !isHandEquipped(name) {
			t.Errorf("%s should be held upright", name)
		}
	}
	for _, name := range flat {
		if isHandEquipped(name) {
			t.Errorf("%s should be held flat", name)
		}
	}
}

func TestIsSlimSkin(t *testing.T) {
	wide := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			wide.SetNRGBA(x, y, color.NRGBA{120, 120, 120, 255})
		}
	}
	if isSlimSkin(wide) {
		t.Error("a fully opaque skin should be wide")
	}

	slim := cloneImage(wide)
	slim.SetNRGBA(50, 16, color.NRGBA{0, 0, 0, 0})
	if !isSlimSkin(slim) {
		t.Error("a transparent unused area should make a skin slim")
	}

	black := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			black.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 255})
		}
	}
	if !isSlimSkin(black) {
		t.Error("all-black unused areas should make a skin slim")
	}

	hd := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			hd.SetNRGBA(x, y, color.NRGBA{120, 120, 120, 255})
		}
	}
	if isSlimSkin(hd) {
		t.Error("a fully opaque HD skin should be wide")
	}
	hd.SetNRGBA(100, 32, color.NRGBA{0, 0, 0, 0}) // (50,16) scaled by 2
	if !isSlimSkin(hd) {
		t.Error("a transparent unused area on an HD skin should make it slim")
	}
}

func cloneImage(src *image.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

func isRed(img image.Image) bool {
	r, g, b, _ := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	return r > 0xf000 && g < 0x1000 && b < 0x1000
}

func isGreen(img image.Image) bool {
	r, g, b, _ := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	return g > 0xf000 && r < 0x1000 && b < 0x1000
}

func isBlue(img image.Image) bool {
	r, g, b, _ := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	return b > 0xf000 && r < 0x1000 && g < 0x1000
}

func isYellow(img image.Image) bool {
	r, g, b, _ := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	return r > 0xf000 && g > 0xf000 && b < 0x1000
}
