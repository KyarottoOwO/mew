# MEW

![MEW Logo](frontend/public/logo.png)

**Minecraft Bedrock Texture Pack Manager**

---

MEW is a small, easy-to-use program made in Go that helps you manage Minecraft Bedrock texture packs.
It can quickly convert Java texture packs to Bedrock and even lets you recolor textures with just a few clicks.
Everything runs on your own computer, so your files stay private.

---

## Pack Porter

Upload a `.zip` or `.rar` Java texture pack, or paste a MediaFire link.
MEW will automatically convert it to a Bedrock `.mcpack` file using SwimPorter.

## Multi-Pack Porter

Paste a MediaFire folder link to download and convert every pack in it one by one,
or upload a `.zip` / `.rar` archive containing multiple packs.
Each pack is converted and saved immediately so nothing is lost if you cancel halfway.

## Recolor Tool

Upload a `.mcpack` file to open the recolor editor. Browse texture folders using the
tabs, then click any texture to open the editor with Hue, Saturation, and Brightness sliders.

**Paint Mode** — Toggle the paintbrush icon, adjust the sliders to pick your
color, then click any texture to apply it. Each click paints from the original image, so
changing the sliders and clicking again replaces the color instead of stacking.

**Export** — When you're happy with your changes, click "Export Pack" to save a
new `.mcpack` file with all your recolored textures.

## Settings

Configure auto-import to directly inject ported packs into your Minecraft resource packs folder.
Set a custom output directory, enable auto-open on export, and inject a custom manifest description.

## Theme

Switch between **Dark** and **Light** mode from Settings. Your preference is saved and restored on launch.

## Contact

- [Discord](https://discord.gg/AA8MSTDjB) — Join the MEW community
- [Donate](https://www.paypal.me/spotters1) — Support development
