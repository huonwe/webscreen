#!/usr/bin/env python3
"""Generate the WebScreen app icons used by the fnOS package.

The upstream project only ships a small SVG logo, while fnOS requires PNG
icons. This renders the same monitor glyph (plus a blue gradient background)
at the exact sizes the package needs. Standard library only, so it also runs in
CI.
"""

import os
import struct
import sys
import zlib

SS = 4  # supersampling factor (anti-aliasing)
GLYPH_RATIO = 0.56  # the glyph covers 56% of the canvas
BG_TOP = (10, 132, 255)  # #0a84ff
BG_BOTTOM = (0, 100, 210)  # #0064d2


def inside_rounded_rect(px, py, x0, y0, x1, y1, r):
    if px < x0 or px > x1 or py < y0 or py > y1:
        return False
    cx = min(max(px, x0 + r), x1 - r)
    cy = min(max(py, y0 + r), y1 - r)
    dx, dy = px - cx, py - cy
    return dx * dx + dy * dy <= r * r


def sample(px, py, size, glyph_box, scale):
    """Return (r, g, b, a) for one sub-sample, straight (non premultiplied) alpha."""
    # background: rounded square with a diagonal gradient
    if inside_rounded_rect(px, py, 0, 0, size, size, size * 0.22):
        t = min(max((px + py) / (2.0 * size), 0.0), 1.0)
        bg = tuple(BG_TOP[i] + (BG_BOTTOM[i] - BG_TOP[i]) * t for i in range(3))
    else:
        bg = None

    # foreground: the monitor glyph in the SVG's 24x24 coordinate space
    ox, oy = glyph_box
    ux, uy = (px - ox) / scale, (py - oy) / scale
    fg = False
    if 0.0 <= ux <= 24.0 and 0.0 <= uy <= 24.0:
        body = inside_rounded_rect(ux, uy, 2, 3, 22, 18, 1.6)
        hole = 4 <= ux <= 20 and 5 <= uy <= 16
        stand = 6.5 <= ux <= 17.5 and 18 <= uy <= 20
        fg = (body and not hole) or stand

    if fg:
        return (255.0, 255.0, 255.0, 1.0)
    if bg is not None:
        return (bg[0], bg[1], bg[2], 1.0)
    return (0.0, 0.0, 0.0, 0.0)


def render(size):
    glyph = size * GLYPH_RATIO
    ox = oy = (size - glyph) / 2.0
    scale = glyph / 24.0
    rows = []
    for y in range(size):
        row = bytearray()
        for x in range(size):
            ar = ag = ab = aa = 0.0
            for sy in range(SS):
                for sx in range(SS):
                    r, g, b, a = sample(x + (sx + 0.5) / SS, y + (sy + 0.5) / SS, size, (ox, oy), scale)
                    ar += r * a
                    ag += g * a
                    ab += b * a
                    aa += a
            n = SS * SS
            if aa > 0:
                row += bytes((int(ar / aa + 0.5), int(ag / aa + 0.5), int(ab / aa + 0.5), int(aa / n * 255 + 0.5)))
            else:
                row += bytes((0, 0, 0, 0))
        rows.append(bytes(row))
    return rows


def write_png(path, size, rows):
    def chunk(tag, data):
        return struct.pack('>I', len(data)) + tag + data + struct.pack('>I', zlib.crc32(tag + data) & 0xFFFFFFFF)

    raw = b''.join(b'\x00' + r for r in rows)
    png = (b'\x89PNG\r\n\x1a\n'
           + chunk(b'IHDR', struct.pack('>IIBBBBB', size, size, 8, 6, 0, 0, 0))
           + chunk(b'IDAT', zlib.compress(raw, 9))
           + chunk(b'IEND', b''))
    os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
    with open(path, 'wb') as fh:
        fh.write(png)
    print("%-28s %dx%d %d bytes" % (path, size, size, len(png)))


def main():
    if len(sys.argv) > 1:
        base = sys.argv[1]
    else:
        base = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'fnpack')
    cache = {s: render(s) for s in (64, 256)}
    for path, size in (
        (os.path.join(base, 'ICON.PNG'), 64),
        (os.path.join(base, 'ICON_256.PNG'), 256),
        (os.path.join(base, 'app/ui/images/icon_64.png'), 64),
        (os.path.join(base, 'app/ui/images/icon_256.png'), 256),
    ):
        write_png(path, size, cache[size])


if __name__ == '__main__':
    main()
