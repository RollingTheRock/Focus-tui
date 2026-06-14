#!/usr/bin/env python3
"""Generate a pixel-style README hero logo for focus."""

from PIL import Image, ImageDraw, ImageFont
import os

OUTPUT_PATH = "docs/assets/focus-logo.png"
WIDTH, HEIGHT = 1200, 420
SCALE = 16  # render large then downsample for pixelation

# Colors
dark_bg = (13, 17, 23)       # GitHub dark background
primary_text = (201, 209, 217)  # GitHub primary text
accent = (101, 233, 249)     # cyan accent
accent2 = (165, 180, 252)    # lavender accent

# Pixel font: use a monospace font at a large size.
# Try common system monospace fonts.
font_names = [
    "/home/rollingtherock/.local/share/fonts/JetBrainsMono/JetBrainsMonoNerdFontMono-Bold.ttf",
    "/usr/share/fonts/adobe-source-code-pro-fonts/SourceCodePro-Bold.otf",
    "/usr/share/fonts/liberation-mono-fonts/LiberationMono-Bold.ttf",
    "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf",
]
font_path = None
for fn in font_names:
    if os.path.exists(fn):
        font_path = fn
        break

# Canvas at high resolution for pixelation effect
W, H = WIDTH * SCALE, HEIGHT * SCALE
img = Image.new("RGBA", (W, H), dark_bg)
draw = ImageDraw.Draw(img)

if font_path is None:
    raise RuntimeError("No monospace font found")

font_size = int(H * 0.45)
font = ImageFont.truetype(font_path, font_size)

# Measure text
bbox = draw.textbbox((0, 0), "focus", font=font)
text_w = bbox[2] - bbox[0]
text_h = bbox[3] - bbox[1]
x = (W - text_w) // 2
y = (H - text_h) // 2 - int(H * 0.08)

# Draw subtle shadow
shadow_offset = int(font_size * 0.06)
draw.text((x + shadow_offset, y + shadow_offset), "focus", font=font, fill=(48, 54, 61))

# Draw main text
draw.text((x, y), "focus", font=font, fill=primary_text)

# Decorative element: three small squares under the text suggesting parallel agents/worktrees
sq = int(H * 0.045)
gap = int(sq * 0.6)
total_w = 3 * sq + 2 * gap
sx = (W - total_w) // 2
sy = y + text_h + int(H * 0.12)

colors = [accent, accent2, (255, 255, 255)]
for i, col in enumerate(colors):
    draw.rectangle([sx + i * (sq + gap), sy, sx + i * (sq + gap) + sq, sy + sq], fill=col)

# Downsample with nearest neighbor for pixelated look
img_small = img.resize((WIDTH, HEIGHT), Image.NEAREST)

os.makedirs(os.path.dirname(OUTPUT_PATH), exist_ok=True)
img_small.save(OUTPUT_PATH)
print(f"Saved {OUTPUT_PATH} ({WIDTH}x{HEIGHT})")
