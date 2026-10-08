package main

import (
	"embed"
)

//go:embed assets/loading_messages.json assets/splashes.json
var promoAssets embed.FS

const (
	promoLoadingMessagesFile = "loading_messages.json"
	promoSplashesFile        = "splashes.json"
)

func promoFiles() (map[string][]byte, error) {
	files := map[string][]byte{
		promoLoadingMessagesFile: {},
		promoSplashesFile:        {},
	}
	for name := range files {
		data, err := promoAssets.ReadFile("assets/" + name)
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
	return files, nil
}
