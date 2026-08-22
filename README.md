# MEW

![MEW Logo](frontend/public/logo.png)

**Minecraft Bedrock Texture Pack Manager**

---

Looking for a pack porter, or just a tool that makes dealing with Bedrock packs easier?
Your search stops here.

MEW is small, fast, and made in Go. It converts Java texture packs to Bedrock, builds custom
skies, recolors textures, and a lot more — all on your own computer, so your files stay private.

## What MEW offers

- **Pack Porter** — Convert Java texture packs (`.zip` / `.rar`) to Bedrock `.mcpack` files in
  seconds, or just paste a MediaFire link.
- **Multi-Pack Porter** — Port big folders of packs in one go, even straight from a MediaFire
  folder link. Nothing gets lost if you stop halfway; every finished pack is saved immediately.
- **Sky Converter** — Make a Sky turns any 360° sky image into a working Bedrock sky.
  Port a Sky takes a Java sky and converts it for Bedrock.
- **Animated Inventory** — Turn a `.gif` into an animated inventory for Bedrock.
- **Recolor Tool** — Recolor any texture with Hue, Saturation, and Brightness sliders,
  plus a paint mode for touching up individual textures.
- **Pack Viewer** — Preview armor, tools, items, and even skies in 3D without loading
  Minecraft. You can also delete packs you don't want anymore right from the preview.
- **Auto import** — New packs are injected straight into Minecraft's resource packs folder.
  No manual copying.
- **Merge into packs** — Add new sky textures directly into packs you already have installed.
- **Recent Packs** — Everything you've ported, one click away.
- **Dark and Light mode** — Your pick, saved between launches.

---

## Pack Porter

Upload a `.zip` or `.rar` Java texture pack, or paste a MediaFire link.
MEW automatically converts it to a Bedrock `.mcpack` file using SwimPorter.

## Multi-Pack Porter

Paste a MediaFire folder link to download and convert every pack in it one by one,
or upload a `.zip` / `.rar` archive containing multiple packs.
Each pack is converted and saved immediately, so nothing is lost if you cancel halfway.

## Sky Converter

Two modes, both dead simple:

- **Make a Sky** — take an image and turn it into a Minecraft Bedrock sky. Pick your quality
  (512px, 1024px, or 2048px) and you're done.
- **Port a Sky** — take a Java sky and turn it into a Minecraft Bedrock sky. The faces get cut
  and mapped correctly for you.

Either way you get a ready-to-use `.mcpack`, or you can merge it straight into packs you
already have installed.

## Animated Inventory

Upload a `.gif` to turn it into an animated inventory for Minecraft Bedrock.
Pick a frame duration and empty-area fill, optionally add an overlay `.png`,
then create a `.mcpack` — or install it straight into an existing pack in Minecraft.

## Recolor Tool

Upload a `.mcpack` file to open the recolor editor. Browse texture folders using the
tabs, then click any texture to open the editor with Hue, Saturation, and Brightness sliders.

**Paint Mode** — Toggle the paintbrush icon, adjust the sliders to pick your color,
then click any texture to apply it. Each click paints from the original image, so changing
the sliders and clicking again replaces the color instead of stacking it.

**Export** — When you're happy with your changes, click "Export Pack" to save a new
`.mcpack` file with all your recolored textures.

## Pack Viewer

Browse your installed resource packs and preview armor, tools, and items on a full player
model — without launching Minecraft. Supports custom skins, material tier switching, idle
and walk animations, orbit camera controls, and sky previews. Found a pack you don't want?
Delete it right from the preview window.

## Settings

Configure auto-import to inject ported packs directly into your Minecraft resource packs
folder. Set a custom output directory, enable auto-open on export, and set a custom
manifest description.

## Contact

- [Discord](https://discord.gg/AA8MSTDjB) — Join the MEW community
- [Donate](https://www.paypal.me/spotters1) — Support development
