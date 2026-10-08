package main

import (
	"fmt"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// vanillaRenderReady is emitted after the renderer's vanilla textures finish
// prefetching, so a viewer that drew while some were missing can redraw.
const vanillaRenderReady = "vanillaRenderReady"

// vanillaRenderAssets lists every vanilla texture the skin renderer can ask for:
// all allow-listed items, every armor material's two layers, and the elytra. It
// is derived from the same allow-lists the renderer uses, so the two cannot
// drift apart.
func vanillaRenderAssets() []string {
	assets := make([]string, 0, len(vanillaItemNames)+len(vanillaArmorMaterials)*2+1)
	for name := range vanillaItemNames {
		assets = append(assets, "textures/items/"+name+".png")
	}
	for material := range vanillaArmorMaterials {
		for _, layer := range []int{1, 2} {
			assets = append(assets, fmt.Sprintf("textures/models/armor/%s_%d.png", material, layer))
		}
	}
	assets = append(assets, "textures/models/armor/elytra.png")
	return assets
}

// prefetchVanillaRenderAssets downloads the renderer's vanilla textures in the
// background so the first time a user equips one it is already on disk. Best
// effort: readVanillaFile caches each file and a failure here just means the
// texture is fetched on demand later (and not cached as missing). Nothing
// vanilla is bundled; this only warms the same on-disk cache.
func (a *App) prefetchVanillaRenderAssets() {
	assets := vanillaRenderAssets()
	workers := min(vanillaWorkers, len(assets))
	if workers < 1 {
		return
	}
	jobs := make(chan string)
	var wg sync.WaitGroup
	before := vanillaGeneration()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rel := range jobs {
				if _, err := a.readVanillaFile(rel); err != nil {
					a.logDebug(fmt.Sprintf("prefetch %s: %v", rel, err))
				}
			}
		}()
	}
	for _, rel := range assets {
		jobs <- rel
	}
	close(jobs)
	wg.Wait()
	// If anything actually downloaded, tell the UI: a viewer open during
	// prefetch may have drawn a placeholder for a texture that is now here.
	if vanillaGeneration() != before && a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, vanillaRenderReady, map[string]interface{}{"ready": true})
	}
}
