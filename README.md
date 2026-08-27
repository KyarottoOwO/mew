# MEW

![MEW Logo](frontend/public/logo.png)

**Minecraft Bedrock Texture Pack Manager**

A desktop app built with [Wails](https://wails.io/) (Go + Vue 3) for converting, managing, and customizing Minecraft Bedrock texture packs.

## Features

- **Pack Porter** — Convert Java texture packs (`.zip` / `.rar`) to Bedrock `.mcpack` files, or paste a MediaFire link
- **Multi-Pack Porter** — Batch convert entire folders of packs at once, including MediaFire folder links
- **Sky Converter** — Turn any 360° sky image into a working Bedrock sky, or port a Java sky directly
- **Animated Inventory** — Convert a `.gif` into an animated inventory for Bedrock
- **Recolor Tool** — Recolor textures with hue/saturation/brightness sliders and paint mode
- **Pack Viewer** — Preview armor, tools, items, and skies in 3D without launching Minecraft
- **Auto-import** — Inject ported packs straight into Minecraft's resource packs folder
- **Discord Rich Presence** — Shows your current activity on Discord

## Download

Grab the latest release from the [Releases](https://github.com/KyarottoOwO/mew/releases) page.

## Build from Source

### Prerequisites

- [Go](https://go.dev/dl/) 1.26+
- [Node.js](https://nodejs.org/) 20+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2.13

### Build

```bash
# Install frontend dependencies
cd frontend && npm install && cd ..

# Build
wails build
```

The executable will be at `build/bin/mew.exe`.

### Development

```bash
wails dev
```

## Tech Stack

- **Backend:** Go
- **Frontend:** Vue 3 + Vite
- **Framework:** [Wails](https://wails.io/) v2
- **Porting engine:** [SwimPorter](https://github.com/SIsilicon/SwimPorter)

## Contact

- [Discord](https://discord.gg/nv9GrqTVM3)
- [Donate](https://www.paypal.me/spotters1)
