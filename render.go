package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	bedrockskin "github.com/THEBOSS9345/bedrock-skin-go"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/woozymasta/tga"
)

// maxTextureEdge bounds every texture the render service decodes. HD packs
// ship 32 or 64 pixel armor and item textures; anything far bigger is not one.
const maxTextureEdge = 1024

// mewDefaultSkin is MEW's own fallback skin, used when neither the pack nor
// the user's saved default skin provides one. It is MEW's asset, not game art.
//
//go:embed frontend/src/assets/default-skin.png
var mewDefaultSkin []byte

var (
	mewDefaultSkinOnce sync.Once
	mewDefaultSkinImg  image.Image
)

// itemNameRe is the shape of an item texture name a pack may be asked for.
var itemNameRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// vanillaItemNames are the items a vanilla texture may be fetched for. A name
// outside this set never becomes a network request, however it was spelled.
var vanillaItemNames = map[string]bool{}

func init() {
	for _, tier := range []string{"wood", "stone", "iron", "gold", "diamond", "netherite"} {
		for _, kind := range []string{"sword", "pickaxe", "axe", "shovel", "hoe"} {
			vanillaItemNames[tier+"_"+kind] = true
		}
	}
	for _, name := range []string{
		"stick", "bone", "blaze_rod", "fishing_rod", "mace", "bow", "bread",
		"apple", "cooked_beef", "apple_golden", "ender_pearl", "totem",
	} {
		vanillaItemNames[name] = true
	}
}

// itemAliases maps a friendly name to a wholly different Bedrock texture name.
// Bedrock ships the golden apple's sprite as "apple_golden", so a request for
// "golden_apple" would 404 and the item would silently not render.
var itemAliases = map[string]string{
	"golden_apple": "apple_golden",
}

// itemNameVariants returns the texture names to try for one requested item,
// most-specific first. Java-style tier prefixes ("wooden_axe", "golden_hoe")
// are mapped to Bedrock's own ("wood_axe", "gold_hoe"); everything else is
// tried as given, then by alias.
func itemNameVariants(name string) []string {
	variants := []string{name}
	if a, ok := itemAliases[name]; ok {
		variants = append(variants, a)
	}
	for _, pair := range [][2]string{{"wooden_", "wood_"}, {"golden_", "gold_"}} {
		if strings.HasPrefix(name, pair[0]) {
			variants = append(variants, pair[1]+strings.TrimPrefix(name, pair[0]))
		}
	}
	return variants
}

// RenderRequest describes one render of the player. Zero values are sensible:
// no pack, no armor, no items, the front body view at 512px.
type RenderRequest struct {
	Pack      string             `json:"pack"`      // dir name; "" uses no pack (vanilla + default skin)
	Model     string             `json:"model"`     // "auto", "wide", "slim"
	Material  string             `json:"material"`  // "" or "none" for no armor, else a material
	Elytra    bool               `json:"elytra"`    // an elytra in place of the chestplate
	Right     HandRequest        `json:"right"`     // the right hand's item
	Left      HandRequest        `json:"left"`      // the left hand's item
	View      string             `json:"view"`      // body, chest, head, avatar
	Angle     string             `json:"angle"`     // front, iso; ignored when Camera is set
	Camera    *CameraRequest     `json:"camera"`    // explicit camera; overrides Angle
	Size      int                `json:"size"`      // clamp 32..1024, default 512
	ModelSize float64            `json:"modelSize"` // Scale.Model, 0 = 1
	Parts     map[string]float64 `json:"parts"`     // Scale.Parts, restricted to the six body parts
	HideSkin  bool               `json:"hideSkin"`  // equipment only
	// OverrideSkin uses the user's uploaded skin instead of the pack's own
	// player skin. The viewer sets it when the user picks a skin with "Change
	// Skin", so that choice is not silently ignored by a pack that ships one.
	OverrideSkin bool   `json:"overrideSkin"`
	Animation    string `json:"animation"` // "" for a still; else a Motion name or ExampleAnimations key
	Frame        int    `json:"frame"`     // which frame of Animation to draw; wrapped into range
	FPS          int    `json:"fps"`       // clamp 1..30, default 15
	Frames       int    `json:"frames"`    // 0 = one loop; clamp to 120
}

// HandRequest is one hand's item and its optional manual adjustment.
type HandRequest struct {
	Item   string                 `json:"item"`   // "" holds nothing
	Adjust bedrockskin.ItemAdjust `json:"adjust"` // zero = the game's placement
}

// CameraRequest is an explicit camera, e.g. for the live view.
type CameraRequest struct {
	Yaw    float64 `json:"yaw"`
	Pitch  float64 `json:"pitch"`
	FOV    float64 `json:"fov"`
	Margin float64 `json:"margin"`
}

// allowedRenderParts are the only bone names a request may scale.
var allowedRenderParts = map[string]bool{
	"head": true, "body": true, "rightarm": true, "leftarm": true, "rightleg": true, "leftleg": true,
}

// --- texture resolution ---

// skinPath is the pack's player skin, or "" when it does not override one.
func (a *App) skinPath(packName string) string {
	if packName == "" {
		return ""
	}
	return findPackSkin(a.getPackDir(packName))
}

// packArmorPath is the pack's texture for one armor layer, or "".
func (a *App) packArmorPath(packName string, material string, layer int) string {
	base := fmt.Sprintf("%s_%d", material, layer)
	return findFirstImage(filepath.Join(a.getPackDir(packName), "textures", "models", "armor"), base)
}

// packElytraPath is the pack's elytra texture, or "".
func (a *App) packElytraPath(packName string) string {
	return findFirstImage(filepath.Join(a.getPackDir(packName), "textures", "models", "armor"), "elytra")
}

// packItemPath is the pack's texture for an item, or "".
func (a *App) packItemPath(packName string, name string) string {
	return findFirstImage(filepath.Join(a.getPackDir(packName), "textures", "items"), name)
}

// defaultSkinPath is the user's saved default skin.
func defaultSkinPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "mew", "default_skin.png")
}

// defaultSkin returns the user's uploaded skin, else MEW's own embedded one. It
// is the fallback for a pack without a skin, and the override when the viewer
// asks to use the user's skin explicitly. It never returns nil.
func (a *App) defaultSkin() image.Image {
	if p := defaultSkinPath(); p != "" {
		if img := a.loadFileTexture(p); img != nil {
			return img
		}
	}
	return mewSkinImage()
}

// skinFor returns the player skin to draw: the pack's, else the user's
// default skin, else MEW's own embedded one. It never returns nil.
func (a *App) skinFor(packName string) image.Image {
	if p := a.skinPath(packName); p != "" {
		if img := a.loadFileTexture(p); img != nil {
			return img
		}
	}
	return a.defaultSkin()
}

// armorLayer returns one armor layer's texture: the pack's, else vanilla's for
// a material vanilla ships. It returns nil for anything else.
func (a *App) armorLayer(packName string, material string, layer int) image.Image {
	if packName != "" {
		if p := a.packArmorPath(packName, material, layer); p != "" {
			if img := a.loadFileTexture(p); img != nil {
				return img
			}
		}
	}
	if vanillaArmorMaterials[material] {
		return a.loadVanillaTexture(fmt.Sprintf("textures/models/armor/%s_%d.png", material, layer))
	}
	return nil
}

// elytraTexture returns the elytra's texture: the pack's, else vanilla's.
func (a *App) elytraTexture(packName string) image.Image {
	if packName != "" {
		if p := a.packElytraPath(packName); p != "" {
			if img := a.loadFileTexture(p); img != nil {
				return img
			}
		}
	}
	return a.loadVanillaTexture("textures/models/armor/elytra.png")
}

// itemTexture returns an item's sprite: the pack's texture for that name when
// it has one, else vanilla's for the names vanilla ships. A name that is not
// a valid texture name, or whose variants are not in the vanilla allow-list,
// returns nil without ever fetching anything. Variants map friendly/Java names
// to Bedrock's own (see itemNameVariants); the pack is checked under each.
func (a *App) itemTexture(packName string, name string) image.Image {
	if !itemNameRe.MatchString(name) {
		return nil
	}
	variants := itemNameVariants(name)
	if packName != "" {
		for _, n := range variants {
			if p := a.packItemPath(packName, n); p != "" {
				if img := a.loadFileTexture(p); img != nil {
					return img
				}
			}
		}
	}
	for _, n := range variants {
		if vanillaItemNames[n] {
			return a.loadVanillaTexture("textures/items/" + n + ".png")
		}
	}
	return nil
}

// loadFileTexture decodes an image file, cached by path, size and mtime so an
// edit in the Recolor tool is picked up.
func (a *App) loadFileTexture(path string) image.Image {
	if path == "" {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	key := fmt.Sprintf("tex\x00%s\x00%d\x00%d", path, st.Size(), st.ModTime().UnixNano())
	if v, ok := a.thumbCache.Load(key); ok {
		if img, ok := v.(image.Image); ok {
			return img
		}
		return nil
	}
	img, err := decodeImageFile(path)
	if err != nil {
		a.logDebug(fmt.Sprintf("mew: texture %s: %v", path, err))
		a.thumbCache.Store(key, missingTexture{})
		return nil
	}
	a.thumbCache.Store(key, img)
	return img
}

// loadVanillaTexture decodes an immutable vanilla texture, cached by its
// resource path. Only a definitive 404 is cached as missing; a transient
// failure is retried, so a download that fails mid-run does not poison the
// texture for the rest of the session.
func (a *App) loadVanillaTexture(rel string) image.Image {
	key := "vtex\x00" + rel
	if v, ok := a.thumbCache.Load(key); ok {
		img, _ := v.(image.Image)
		return img
	}
	data, err := a.readVanillaFile(rel)
	if err != nil {
		a.logDebug(fmt.Sprintf("mew: vanilla texture %s: %v", rel, err))
		// Only a definitive 404 is cached as missing. A transient failure must
		// not poison the texture for the rest of the session.
		if errors.Is(err, errVanillaNotFound) {
			a.thumbCache.Store(key, missingTexture{})
		}
		return nil
	}
	img, err := decodeTexture(data)
	if err != nil {
		a.logDebug(fmt.Sprintf("mew: vanilla texture %s: %v", rel, err))
		a.thumbCache.Store(key, missingTexture{})
		return nil
	}
	a.thumbCache.Store(key, img)
	return img
}

// missingTexture marks a cache entry as a texture that could not be loaded.
type missingTexture struct{}

// decodeImageFile decodes a pack image, handling the TGA files packs ship
// alongside PNGs.
func decodeImageFile(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".tga") {
		return tga.Decode(bytes.NewReader(data))
	}
	return decodeTexture(data)
}

// decodeTexture decodes encoded image bytes, bounding the edge length first so
// a hostile header cannot ask for a huge allocation.
func decodeTexture(data []byte) (image.Image, error) {
	w, h, err := bedrockskin.ImageDimensions(data)
	if err != nil {
		return nil, err
	}
	if w > maxTextureEdge || h > maxTextureEdge {
		return nil, fmt.Errorf("texture is %dx%d, larger than %d", w, h, maxTextureEdge)
	}
	return bedrockskin.DecodeImage(data)
}

// mewSkinImage decodes MEW's embedded fallback skin once.
func mewSkinImage() image.Image {
	mewDefaultSkinOnce.Do(func() {
		img, err := decodeTexture(mewDefaultSkin)
		if err != nil {
			debugLogf("mew: embedded default skin: %v", err)
			return
		}
		mewDefaultSkinImg = img
	})
	return mewDefaultSkinImg
}

var (
	placeholderOnce sync.Once
	placeholderImg  image.Image
)

// placeholderTexture is MEW's own "missing texture" marker: a magenta and black
// checkerboard. It stands in for an armor piece, item or elytra whose texture
// could not be found in the pack or in vanilla - for example while the vanilla
// cache is empty and offline, or right after it was cleared - so the equipment
// stays visible and obviously provisional instead of silently disappearing. It
// is generated, never bundled game art.
func placeholderTexture() image.Image {
	placeholderOnce.Do(func() {
		const size = 16
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		magenta := color.NRGBA{R: 255, A: 255}
		black := color.NRGBA{A: 255}
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if (x/4+y/4)%2 == 0 {
					img.SetNRGBA(x, y, magenta)
				} else {
					img.SetNRGBA(x, y, black)
				}
			}
		}
		placeholderImg = img
	})
	return placeholderImg
}

// orPlaceholder returns img when a texture resolved, else the missing-texture
// marker, so a render never silently drops a requested piece.
func orPlaceholder(img image.Image) image.Image {
	if img == nil {
		return placeholderTexture()
	}
	return img
}

// --- model detection ---

// slimAreas are the four 2-pixel-wide regions a slim skin leaves unused, in
// the 64x64 layout. A skin is slim when any of them is not fully opaque, or
// all four are entirely black, or all four entirely white - the same test
// skinview3d's "Auto" used, so existing users see the same arms.
var slimAreas = [][4]int{{50, 16, 2, 4}, {54, 20, 2, 12}, {42, 48, 2, 4}, {46, 52, 2, 12}}

// isSlimSkin reports whether a skin's arms are slim. HD skins scale the tested
// areas by width/64.
func isSlimSkin(img image.Image) bool {
	if img == nil {
		return false
	}
	b := img.Bounds()
	if b.Dx() < 64 {
		return false
	}
	scale := float64(b.Dx()) / 64
	transparent, allBlack, allWhite := false, true, true
	for _, area := range slimAreas {
		hasClear, black, white := scanSlimArea(img, b, area, scale)
		if hasClear {
			transparent = true
		}
		if !black {
			allBlack = false
		}
		if !white {
			allWhite = false
		}
	}
	return transparent || allBlack || allWhite
}

// scanSlimArea reports whether an area has a non-opaque pixel, is all black or
// is all white.
func scanSlimArea(img image.Image, b image.Rectangle, area [4]int, scale float64) (hasClear, black, white bool) {
	x0 := int(float64(area[0]) * scale)
	y0 := int(float64(area[1]) * scale)
	w := int(float64(area[2]) * scale)
	h := int(float64(area[3]) * scale)
	black, white = true, true
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if x < 0 || y < 0 || x >= b.Dx() || y >= b.Dy() {
				return true, false, false
			}
			r, g, bl, al := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if al != 0xffff {
				hasClear = true
			}
			if !(r == 0 && g == 0 && bl == 0 && al == 0xffff) {
				black = false
			}
			if !(r == 0xffff && g == 0xffff && bl == 0xffff && al == 0xffff) {
				white = false
			}
		}
	}
	return hasClear, black, white
}

// --- held items ---

// handEquippedRe matches the tool and weapon suffixes held upright by the game.
var handEquippedRe = regexp.MustCompile(`_(sword|axe|pickaxe|shovel|hoe|spear)$`)

// handEquippedNames are the non-suffixed items the game holds upright.
var handEquippedNames = map[string]bool{
	"stick": true, "bone": true, "blaze_rod": true, "breeze_rod": true,
	"fishing_rod": true, "carrot_on_a_stick": true, "warped_fungus_on_a_stick": true,
	"mace": true,
}

// isHandEquipped reports whether the game holds an item upright (a tool or
// weapon) rather than flat (food, materials).
func isHandEquipped(name string) bool {
	return handEquippedRe.MatchString(name) || handEquippedNames[name]
}

// --- options ---

// renderOptions turns a request into library options, validating and clamping
// everything that came from the frontend.
func (a *App) renderOptions(req RenderRequest) (bedrockskin.Options, error) {
	if req.Pack != "" && filepath.Base(req.Pack) != req.Pack {
		return bedrockskin.Options{}, fmt.Errorf("invalid pack name")
	}

	skin := a.skinFor(req.Pack)
	if req.OverrideSkin {
		skin = a.defaultSkin()
	}
	if skin == nil {
		return bedrockskin.Options{}, bedrockskin.ErrNoTexture
	}

	model := strings.ToLower(strings.TrimSpace(req.Model))
	if model == "" {
		model = "auto"
	}
	switch model {
	case "auto", "wide", "slim":
	default:
		return bedrockskin.Options{}, fmt.Errorf("unknown model: %s", req.Model)
	}
	identifier := ""
	if model == "slim" || (model == "auto" && isSlimSkin(skin)) {
		identifier = "geometry.humanoid.customSlim"
	}

	view, err := bedrockskin.ParseView(req.View)
	if err != nil {
		return bedrockskin.Options{}, err
	}
	angle, err := bedrockskin.ParseAngle(req.Angle)
	if err != nil {
		return bedrockskin.Options{}, err
	}
	parts, err := validateParts(req.Parts)
	if err != nil {
		return bedrockskin.Options{}, err
	}

	opts := bedrockskin.Options{
		Texture:    skin,
		Identifier: identifier,
		View:       view,
		Angle:      angle,
		Size:       clampInt(req.Size, 32, 1024, 512),
		Scale:      bedrockskin.Scale{Model: req.ModelSize, Parts: parts},
		HideSkin:   req.HideSkin,
	}
	if req.Camera != nil {
		opts.Camera = cameraFromRequest(req.Camera)
	}

	material := strings.ToLower(strings.TrimSpace(req.Material))
	if material == "none" {
		material = ""
	}
	if material != "" {
		if !vanillaArmorMaterials[material] {
			return bedrockskin.Options{}, fmt.Errorf("unknown armor material: %s", req.Material)
		}
		opts.Armor = bedrockskin.ArmorSet(
			orPlaceholder(a.armorLayer(req.Pack, material, 1)),
			orPlaceholder(a.armorLayer(req.Pack, material, 2)),
		)
	}
	if req.Elytra {
		opts.Armor.Elytra = orPlaceholder(a.elytraTexture(req.Pack))
	}

	if opts.RightHand, err = a.heldFor(req.Pack, req.Right); err != nil {
		return bedrockskin.Options{}, err
	}
	if opts.LeftHand, err = a.heldFor(req.Pack, req.Left); err != nil {
		return bedrockskin.Options{}, err
	}

	// Animation is applied by the frame paths (RenderSkin frames, RenderSkinGIF);
	// a posed still is only ever drawn with the animation's shared camera, so it
	// goes through renderAnimationFrame rather than a per-pose here.
	return opts, nil
}

// cameraFromRequest validates a request's explicit camera, or nil when it has
// none.
func cameraFromRequest(c *CameraRequest) *bedrockskin.Camera {
	if c == nil {
		return nil
	}
	return &bedrockskin.Camera{
		Yaw:    c.Yaw,
		Pitch:  clampFloat(c.Pitch, -89, 89),
		FOV:    nonzeroClamp(c.FOV, 10, 90),
		Margin: nonzeroClamp(c.Margin, 0.3, 4),
	}
}

// heldFor turns a hand request into library held-item options.
func (a *App) heldFor(packName string, h HandRequest) (bedrockskin.Held, error) {
	name := strings.TrimSpace(h.Item)
	if name == "" {
		return bedrockskin.Held{}, nil
	}
	if !itemNameRe.MatchString(name) {
		return bedrockskin.Held{}, fmt.Errorf("invalid item name: %s", h.Item)
	}
	return bedrockskin.Held{
		Item:   orPlaceholder(a.itemTexture(packName, name)),
		Flat:   !isHandEquipped(name),
		Adjust: h.Adjust,
	}, nil
}

// validateParts checks a request's part scales against the six body parts,
// returning the map unchanged when valid.
func validateParts(parts map[string]float64) (map[string]float64, error) {
	for name := range parts {
		if !allowedRenderParts[strings.ToLower(strings.TrimSpace(name))] {
			return nil, fmt.Errorf("unknown body part: %s", name)
		}
	}
	return parts, nil
}

// animationFor resolves an animation name: a built-in motion or one of the
// bundled example animations. A blank name returns nil, meaning a still.
func animationFor(name string) (bedrockskin.Animator, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	if m, err := bedrockskin.ParseMotion(name); err == nil {
		return m, nil
	}
	if anim := bedrockskin.ExampleAnimations()[name]; anim != nil {
		return anim, nil
	}
	return nil, fmt.Errorf("unknown animation: %s", name)
}

// --- endpoints ---

// RenderSkin renders one PNG of the player with the requested equipment,
// returning it as a data URI. With Animation set it draws Frame of that
// animation, framed by the one camera the whole animation shares, so whole-body
// motion keeps its place on screen.
func (a *App) RenderSkin(req RenderRequest) (string, error) {
	key := a.renderCacheKey(req)
	if uri, ok := a.renderCache.get(key); ok {
		return uri, nil
	}
	png, err := func() ([]byte, error) {
		if strings.TrimSpace(req.Animation) != "" {
			return a.renderAnimationFrame(req)
		}
		opts, err := a.renderOptions(req)
		if err != nil {
			return nil, err
		}
		return opts.RenderPNG()
	}()
	if err != nil {
		return "", err
	}
	uri := pngDataURI(png)
	a.renderCache.put(key, uri)
	return uri, nil
}

// renderAnimationFrame draws one frame of an animation with the animation's
// shared camera - the mid-drag still the live view uses.
func (a *App) renderAnimationFrame(req RenderRequest) ([]byte, error) {
	frames, err := a.preparedFrames(req)
	if err != nil {
		return nil, err
	}
	img := frames.Draw(req.Frame, clampInt(req.Size, 32, 1024, 512), cameraFromRequest(req.Camera))
	return bedrockskin.EncodePNG(img)
}

// preparedFrames returns the animation's prepared frames, building and caching
// them on the first request. The camera is refit per draw, so it is not part of
// the cache key.
func (a *App) preparedFrames(req RenderRequest) (*bedrockskin.Frames, error) {
	anim, err := animationFor(req.Animation)
	if err != nil {
		return nil, err
	}
	if anim == nil {
		return nil, fmt.Errorf("no animation requested")
	}
	key := a.framesCacheKey(req)
	if set := a.framesCache.get(key); set != nil {
		return set, nil
	}
	opts, err := a.renderOptions(req)
	if err != nil {
		return nil, err
	}
	set, err := bedrockskin.PrepareFrames(bedrockskin.AnimationOptions{
		Options:   opts,
		Animation: anim,
		FPS:       clampInt(req.FPS, 1, 30, 15),
		Frames:    clampInt(req.Frames, 0, 120, 0),
	})
	if err != nil {
		return nil, err
	}
	a.framesCache.put(key, set)
	return set, nil
}

// RenderSkinFrames renders every frame of an animation, each framed by one
// camera, as PNG data URIs. The live view loops these.
func (a *App) RenderSkinFrames(req RenderRequest) ([]string, error) {
	anim, err := animationFor(req.Animation)
	if err != nil {
		return nil, err
	}
	if anim == nil {
		return nil, fmt.Errorf("no animation requested")
	}
	frames, err := a.preparedFrames(req)
	if err != nil {
		return nil, err
	}
	// The whole clip is re-drawn for the camera it was requested at, then
	// encoded. Drawing reuses the prepared poses (no per-frame re-posing) and
	// the PNG encode runs across cores, so reloading a clip after a rotation is
	// a short burst instead of a long serial one.
	n := frames.Len()
	out := make([]string, n)
	if n == 0 {
		return out, nil
	}
	size := clampInt(req.Size, 32, 1024, 512)
	cam := cameraFromRequest(req.Camera)
	workers := min(runtime.GOMAXPROCS(0), n)
	next := make(chan int, n)
	for i := range out {
		next <- i
	}
	close(next)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		fErr error
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				png, err := bedrockskin.EncodePNG(frames.Draw(i, size, cam))
				if err != nil {
					mu.Lock()
					if fErr == nil {
						fErr = err
					}
					mu.Unlock()
					continue
				}
				out[i] = pngDataURI(png)
			}
		}()
	}
	wg.Wait()
	if fErr != nil {
		return nil, fErr
	}
	return out, nil
}

// RenderSkinGIF renders an animation as a looping GIF data URI, for export.
func (a *App) RenderSkinGIF(req RenderRequest) (string, error) {
	anim, err := animationFor(req.Animation)
	if err != nil {
		return "", err
	}
	if anim == nil {
		return "", fmt.Errorf("no animation requested")
	}
	opts, err := a.renderOptions(req)
	if err != nil {
		return "", err
	}
	if opts.Size > 512 {
		opts.Size = 512
	}
	gif, err := bedrockskin.RenderGIF(bedrockskin.AnimationOptions{
		Options:   opts,
		Animation: anim,
		FPS:       clampInt(req.FPS, 1, 30, 15),
		Frames:    clampInt(req.Frames, 0, 120, 0),
	})
	if err != nil {
		return "", err
	}
	return "data:image/gif;base64," + base64.StdEncoding.EncodeToString(gif), nil
}

// ListAnimations returns the motions first, then the bundled example
// animations sorted by name.
func (a *App) ListAnimations() []string {
	motions := bedrockskin.Motions()
	out := make([]string, 0, len(motions)+len(bedrockskin.ExampleAnimations()))
	for _, m := range motions {
		out = append(out, string(m))
	}
	examples := bedrockskin.ExampleAnimations()
	names := make([]string, 0, len(examples))
	for name := range examples {
		names = append(names, name)
	}
	sort.Strings(names)
	return append(out, names...)
}

// RenderItem renders one item on its own, extruded, front or iso.
func (a *App) RenderItem(pack string, item string, angle string, size int) (string, error) {
	img := orPlaceholder(a.itemTexture(pack, strings.TrimSpace(item)))
	parsedAngle, err := bedrockskin.ParseAngle(angle)
	if err != nil {
		return "", err
	}
	out, err := bedrockskin.RenderItem(bedrockskin.ItemOptions{
		Item:  img,
		Angle: parsedAngle,
		Size:  clampInt(size, 32, 512, 256),
	})
	if err != nil {
		return "", err
	}
	png, err := bedrockskin.EncodePNG(out)
	if err != nil {
		return "", err
	}
	return pngDataURI(png), nil
}

// RenderItemSpin renders one item turning once, as a GIF data URI.
func (a *App) RenderItemSpin(pack string, item string, size int) (string, error) {
	img := orPlaceholder(a.itemTexture(pack, strings.TrimSpace(item)))
	gif, err := bedrockskin.RenderItemGIF(bedrockskin.ItemAnimationOptions{
		ItemOptions: bedrockskin.ItemOptions{Item: img, Size: clampInt(size, 32, 512, 256)},
	})
	if err != nil {
		return "", err
	}
	return "data:image/gif;base64," + base64.StdEncoding.EncodeToString(gif), nil
}

// SaveRender writes a rendered data URI to a file the user picks. A cancelled
// dialog is not an error.
func (a *App) SaveRender(dataURI string, suggestedName string) error {
	mime, data, err := splitDataURI(dataURI)
	if err != nil {
		return err
	}
	filter := wailsRuntime.FileFilter{DisplayName: "PNG image", Pattern: "*.png"}
	ext := ".png"
	if strings.Contains(mime, "gif") {
		filter = wailsRuntime.FileFilter{DisplayName: "GIF image", Pattern: "*.gif"}
		ext = ".gif"
	}
	name := filepath.Base(strings.TrimSpace(suggestedName))
	if name == "" || name == "." || name == string(os.PathSeparator) {
		name = "render" + ext
	} else if filepath.Ext(name) == "" {
		name += ext
	}
	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "Save render",
		DefaultFilename: name,
		Filters:         []wailsRuntime.FileFilter{filter},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	return os.WriteFile(path, data, 0644)
}

// --- helpers ---

// pngDataURI encodes an image as a PNG data URI.
func pngDataURI(png []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// imageToDataURI encodes a decoded image as a PNG data URI, or "" for nil.
func imageToDataURI(img image.Image) string {
	if img == nil {
		return ""
	}
	png, err := bedrockskin.EncodePNG(img)
	if err != nil {
		return ""
	}
	return pngDataURI(png)
}

// splitDataURI separates a base64 data URI's mime type from its bytes.
func splitDataURI(uri string) (string, []byte, error) {
	if !strings.HasPrefix(uri, "data:") {
		return "", nil, fmt.Errorf("not a data URI")
	}
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return "", nil, fmt.Errorf("malformed data URI")
	}
	meta := uri[len("data:"):comma]
	if !strings.HasSuffix(meta, ";base64") {
		return "", nil, fmt.Errorf("data URI is not base64")
	}
	data, err := base64.StdEncoding.DecodeString(uri[comma+1:])
	if err != nil {
		return "", nil, err
	}
	return strings.TrimSuffix(meta, ";base64"), data, nil
}

// clampInt clamps v into [lo, hi], with zero meaning def.
func clampInt(v, lo, hi, def int) int {
	if v == 0 {
		v = def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// clampFloat clamps v into [lo, hi].
func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// nonzeroClamp clamps v when it is set, leaving zero to mean "the preset".
func nonzeroClamp(v, lo, hi float64) float64 {
	if v == 0 {
		return 0
	}
	return clampFloat(v, lo, hi)
}

// --- render cache ---

// renderCacheKey is a request plus the mtimes of the pack files it read and the
// vanilla generation, so an edited texture or newly downloaded vanilla texture
// re-renders instead of serving a render drawn while it was missing.
func (a *App) renderCacheKey(req RenderRequest) string {
	raw, _ := json.Marshal(req)
	sig := a.requestSignature(req)
	return fmt.Sprintf("%s\x00%s\x00v%d", raw, sig, vanillaGeneration())
}

// framesCacheKey identifies a prepared animation: the appearance and the
// animation itself, not the camera, frame or size, which a draw supplies.
func (a *App) framesCacheKey(req RenderRequest) string {
	req.Camera = nil
	req.Frame = 0
	req.Size = 0
	raw, _ := json.Marshal(req)
	return fmt.Sprintf("%s\x00%s\x00v%d", raw, a.requestSignature(req), vanillaGeneration())
}

// requestSignature fingerprints the pack and default-skin files a request uses.
func (a *App) requestSignature(req RenderRequest) string {
	var b strings.Builder
	statInto(&b, a.skinPath(req.Pack))
	statInto(&b, defaultSkinPath())
	material := strings.ToLower(strings.TrimSpace(req.Material))
	if material == "none" {
		material = ""
	}
	if material != "" {
		statInto(&b, a.packArmorPath(req.Pack, material, 1))
		statInto(&b, a.packArmorPath(req.Pack, material, 2))
	}
	if req.Elytra {
		statInto(&b, a.packElytraPath(req.Pack))
	}
	if req.Right.Item != "" {
		statInto(&b, a.packItemPath(req.Pack, req.Right.Item))
	}
	if req.Left.Item != "" {
		statInto(&b, a.packItemPath(req.Pack, req.Left.Item))
	}
	return b.String()
}

func statInto(b *strings.Builder, path string) {
	if path == "" {
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(b, "%s:absent;", path)
		return
	}
	fmt.Fprintf(b, "%s:%d:%d;", path, st.Size(), st.ModTime().UnixNano())
}

// framesCache keeps the most recent prepared animation. The live view shows one
// appearance at a time, so a single slot is enough to draw every camera move
// from the one frame set instead of rebuilding it.
type framesCache struct {
	mu  sync.Mutex
	key string
	set *bedrockskin.Frames
}

func (c *framesCache) get(key string) *bedrockskin.Frames {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key == key {
		return c.set
	}
	return nil
}

func (c *framesCache) put(key string, set *bedrockskin.Frames) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.key, c.set = key, set
}

// renderLRU is a small least-recently-used cache of rendered stills, so memory
// stays flat while the user drags the camera.
type renderLRU struct {
	mu    sync.Mutex
	max   int
	items map[string]string
	order []string
}

func newRenderLRU(max int) *renderLRU {
	return &renderLRU{max: max, items: make(map[string]string, max)}
}

func (c *renderLRU) get(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[key]
	if !ok {
		return "", false
	}
	c.touch(key)
	return v, true
}

func (c *renderLRU) put(key string, value string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; !ok {
		c.order = append(c.order, key)
	}
	c.items[key] = value
	for len(c.order) > c.max {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
}

// touch moves key to the newest end of the order.
func (c *renderLRU) touch(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}
