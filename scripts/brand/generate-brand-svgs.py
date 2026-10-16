#!/usr/bin/env python3
"""
One-off generator for shared/brand/*.svg (INIT-005/SPEC-006).

Why Python: the Hebrew wordmark must be real outlines (paths), not live text, so the
logo renders the same everywhere. The glyph outlines come from the self-hosted Rubik
files in shared/fonts/ (SIL OFL 1.1 permits this). This script is NOT part of the
Node app and adds no repo dependency. Run it only to revise the brand SVGs:

    python -m venv %TEMP%/hebrew-fonttools
    %TEMP%/hebrew-fonttools/Scripts/pip install fonttools brotli
    %TEMP%/hebrew-fonttools/Scripts/python scripts/brand/generate-brand-svgs.py
    node scripts/build-brand-assets.js        # then rasterize the PNGs

The generated SVG files are committed; nobody needs Python to use them.
"""
import os
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.boundsPen import BoundsPen

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))
BRAND = os.path.join(ROOT, 'shared', 'brand')
FONTS = os.path.join(ROOT, 'shared', 'fonts')

# Palette: shared/worksheet.css :root (--ink, --blue-deep, --brand-1/2/3, --brand-sun, --paper)
INK = '#1e293b'
BLUE = '#1d4ed8'
INDIGO = '#4338ca'
PURPLE = '#7c3aed'
SUN = '#fbbf24'
PAPER = '#faf8f4'
DARK_TEXT = '#f6f3ec'      # ADR dark token --color-text
DARK_ACCENT = '#c7d2fe'    # indigo-200 tint for the wordmark's accent on dark

HE_TEXT = '\u05d3\u05e8\u05da \u05e2\u05d1\u05e8\u05d9\u05ea'   # ×“×¨×š ×¢×‘×¨×™×ª
EN_GLOSS = 'Hebrew Way'


def r(n):
    s = ('%.1f' % n).rstrip('0').rstrip('.')
    return s if s != '-0' else '0'


_font_cache = {}


def font(name, wght):
    key = (name, wght)
    if key not in _font_cache:
        f = TTFont(os.path.join(FONTS, name + '.woff2'))
        f = instantiateVariableFont(f, {'wght': wght}, inplace=False)
        _font_cache[key] = f
    return _font_cache[key]


def glyph_run(name, wght, text, size, rtl, tracking=0.0):
    """Return (path_d, width, height_bounds) for a line of text. Baseline at y=0, y up is negative."""
    f = font(name, wght)
    cmap = f.getBestCmap()
    gs = f.getGlyphSet()
    upm = f['head'].unitsPerEm
    k = size / upm
    chars = list(text)
    order = chars[::-1] if rtl else chars  # logical -> visual left-to-right order
    # Hebrew logical order is right-to-left: first char sits at the right edge.
    x = 0.0
    parts = []
    for ch in order:
        gn = cmap[ord(ch)]
        pen = SVGPathPen(gs, ntos=r)
        tp = TransformPen(pen, (k, 0, 0, -k, x, 0))
        gs[gn].draw(tp)
        d = pen.getCommands()
        if d:
            parts.append(d)
        x += gs[gn].width * k + tracking
    return ' '.join(parts), x - tracking


def glyph_path(name, wght, ch, size):
    """Single glyph outline, plus bounding box, in size-scaled coordinates (y down)."""
    f = font(name, wght)
    gs = f.getGlyphSet()
    gn = f.getBestCmap()[ord(ch)]
    k = size / f['head'].unitsPerEm
    bp = BoundsPen(gs)
    gs[gn].draw(bp)
    xmin, ymin, xmax, ymax = bp.bounds
    return gn, k, (xmin * k, -ymax * k, xmax * k, -ymin * k)


def centered_glyph(wght, ch, size, cx, cy):
    f = font('rubik-hebrew', wght)
    gs = f.getGlyphSet()
    gn, k, (x0, y0, x1, y1) = glyph_path('rubik-hebrew', wght, ch, size)
    tx = cx - (x0 + x1) / 2
    ty = cy - (y0 + y1) / 2
    pen = SVGPathPen(gs, ntos=r)
    gs[gn].draw(TransformPen(pen, (k, 0, 0, -k, tx, ty)))
    return pen.getCommands()


# ---------------------------------------------------------------- marks
# Each mark = (defs, tile_fill, glyph_elements). Glyphs are drawn in a 512 box.

def mark_a():
    d = centered_glyph(700, '\u05e2', 372, 252, 272)
    defs = ('<linearGradient id="a-g" x1="0" y1="0" x2="1" y2="1">'
            '<stop offset="0" stop-color="%s"/><stop offset=".55" stop-color="%s"/>'
            '<stop offset="1" stop-color="%s"/></linearGradient>' % (BLUE, INDIGO, PURPLE))
    glyph = ('<path d="%s" fill="#ffffff"/>'
             '<circle cx="396" cy="124" r="34" fill="%s"/>' % (d, SUN))
    return defs, 'url(#a-g)', glyph


def mark_b():
    # A path that bends twice and ends in the sun dot: a way (×“×¨×š) travelled.
    glyph = ('<path d="M140 392 C140 316 372 352 372 266 C372 188 140 238 140 156" '
             'fill="none" stroke="#ffffff" stroke-width="48" stroke-linecap="round" stroke-linejoin="round"/>'
             '<circle cx="140" cy="112" r="38" fill="%s"/>' % SUN)
    return '', BLUE, glyph


def mark_c():
    # Dalet (×“), first letter of ×“×¨×š, built from two bars; the dot is the next step.
    glyph = ('<rect x="96" y="120" width="320" height="78" rx="22" fill="#ffffff"/>'
             '<rect x="296" y="120" width="80" height="226" rx="22" fill="#ffffff"/>'
             '<circle cx="336" cy="392" r="36" fill="%s"/>' % SUN)
    return '', INDIGO, glyph


MARKS = {'a': mark_a, 'b': mark_b, 'c': mark_c}


def mark_group(key, rounded=True, glyph_scale=1.0):
    """Elements for a 512x512 mark. rounded=False gives a full-bleed square (maskable)."""
    defs, fill, glyph = MARKS[key]()
    rx = ' rx="112"' if rounded else ''
    tile = '<rect id="tile" width="512" height="512"%s fill="%s"/>' % (rx, fill)
    if glyph_scale != 1.0:
        glyph = '<g transform="translate(256 256) scale(%s) translate(-256 -256)">%s</g>' % (glyph_scale, glyph)
    return defs, '%s<g id="glyph">%s</g>' % (tile, glyph)


# ---------------------------------------------------------------- svg writers
def svg_doc(viewbox, body, defs='', title='', extra=''):
    t = '<title>%s</title>' % title if title else ''
    d = '<defs>%s</defs>' % defs if defs else ''
    return ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s" role="img" aria-label="%s"%s>'
            '%s%s%s</svg>\n' % (viewbox, title, extra, t, d, body))


def write(rel, text):
    p = os.path.join(BRAND, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8', newline='\n') as fh:
        fh.write(text)


TITLE = '\u05d3\u05e8\u05da \u05e2\u05d1\u05e8\u05d9\u05ea \u2014 Hebrew Way'

# wordmark styling per option: (hebrew weight, tracking, latin weight, he colour fn)
STYLE = {
    'a': dict(he_w=700, track=0, en_w=500),
    'b': dict(he_w=500, track=2, en_w=500),
    'c': dict(he_w=800, track=4, en_w=600),
}


def wordmark_parts(key, dark):
    s = STYLE[key]
    he_size, en_size = 120, 34
    he_d, he_w = glyph_run('rubik-hebrew', s['he_w'], HE_TEXT, he_size, True, s['track'])
    en_d, en_w = glyph_run('rubik-latin', s['en_w'], EN_GLOSS, en_size, False, 2.5)
    ink = DARK_TEXT if dark else INK
    accent = DARK_ACCENT if dark else (BLUE if key == 'b' else INK)
    return he_d, he_w, en_d, en_w, ink, accent, he_size, en_size


def wordmark_svg(key, dark=False):
    he_d, he_w, en_d, en_w, ink, accent, he_size, en_size = wordmark_parts(key, dark)
    pad = 24
    w = max(he_w, en_w)
    he_base = pad + he_size * 0.80
    en_base = he_base + 18 + en_size * 0.74
    H = en_base + pad + 6
    W = w + 2 * pad
    he_x = pad + (w - he_w)          # right-aligned (RTL)
    en_x = pad + (w - en_w)
    body = ('<g id="wordmark">'
            '<path id="he" transform="translate(%s %s)" d="%s" fill="%s"/>'
            '<path id="en" transform="translate(%s %s)" d="%s" fill="%s"/></g>'
            % (r(he_x), r(he_base), he_d, accent if key == 'b' else ink,
               r(en_x), r(en_base), en_d, ink))
    return svg_doc('0 0 %s %s' % (r(W), r(H)), body, title=TITLE), W, H


def logo_svg(key, dark=False):
    he_d, he_w, en_d, en_w, ink, accent, he_size, en_size = wordmark_parts(key, dark)
    defs, marks = mark_group(key)
    pad = 24
    w = max(he_w, en_w)
    if key == 'c':
        # stacked, centred
        mark = 176
        W = w + 2 * pad
        mx = (W - mark) / 2
        he_base = pad + mark + 26 + he_size * 0.80
        en_base = he_base + 18 + en_size * 0.74
        H = en_base + pad + 6
        he_x = (W - he_w) / 2
        en_x = (W - en_w) / 2
    else:
        mark = 176
        gap = 30
        W = pad + w + gap + mark + pad
        text_block_h = he_size * 0.80 + 18 + en_size * 0.74
        H = max(mark, text_block_h) + 2 * pad + 6
        top = (H - text_block_h) / 2 + 2
        he_base = top + he_size * 0.80
        en_base = he_base + 18 + en_size * 0.74
        mx = W - pad - mark
        he_x = pad + (w - he_w)
        en_x = pad + (w - en_w)
    my = pad if key == 'c' else (H - mark) / 2
    he_fill = accent if key == 'b' else ink
    body = ('<g id="icon" transform="translate(%s %s) scale(%s)">%s</g>'
            '<g id="wordmark">'
            '<path id="he" transform="translate(%s %s)" d="%s" fill="%s"/>'
            '<path id="en" transform="translate(%s %s)" d="%s" fill="%s"/></g>'
            % (r(mx), r(my), '%.4f' % (mark / 512.0), marks,
               r(he_x), r(he_base), he_d, he_fill, r(en_x), r(en_base), en_d, ink))
    return svg_doc('0 0 %s %s' % (r(W), r(H)), body, defs=defs, title=TITLE)


def mark_svg(key):
    defs, marks = mark_group(key)
    return svg_doc('0 0 512 512', marks, defs=defs, title=TITLE)


def favicon_svg(key):
    defs, marks = mark_group(key)
    return svg_doc('0 0 512 512', marks, defs=defs, title=TITLE)


def maskable_svg(key):
    """Full-bleed square, glyph pulled into the maskable safe zone (inner 80% circle)."""
    defs, marks = mark_group(key, rounded=False, glyph_scale=0.8)
    return svg_doc('0 0 512 512', marks, defs=defs, title=TITLE)


# ---------------------------------------------------------------- icons (24 grid, 2px stroke)
ICONS = {
    'play': '<path d="M8 5.5v13l11-6.5z"/>',
    'replay': '<path d="M3.5 12a8.5 8.5 0 1 0 2.6-6.1"/><path d="M3.5 4v4.5H8"/><path d="M10.5 9v6l4.5-3z"/>',
    'next': '<path d="M4 12h15"/><path d="m13 6 6 6-6 6"/>',
    'back': '<path d="M20 12H5"/><path d="m11 6-6 6 6 6"/>',
    'check': '<path d="m5 12.5 4.5 4.5L19 7"/>',
    'wrong': '<path d="m6 6 12 12"/><path d="M18 6 6 18"/>',
    'retry': '<path d="M20 12a8 8 0 1 0-2.3 5.7"/><path d="M20 4v5.5h-5.5"/>',
    'today': '<circle cx="12" cy="14" r="4"/><path d="M12 5.5v2.5"/><path d="m5.6 8.6 1.8 1.8"/><path d="m18.4 8.6-1.8 1.8"/><path d="M3.5 14H6"/><path d="M18 14h2.5"/><path d="M5 20h14"/>',
    'home': '<path d="m3.5 11.5 8.5-7.5 8.5 7.5"/><path d="M6 10v10h4.5v-6h3v6H18V10"/>',
    'lessons': '<path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v15H6.5A2.5 2.5 0 0 0 4 20.5z"/><path d="M4 20.5A2.5 2.5 0 0 1 6.5 18H20v3H6.5A2.5 2.5 0 0 1 4 20.5z"/>',
    'review': '<path d="M4 12a8 8 0 0 1 14-5.3l2 2.3"/><path d="M20 12a8 8 0 0 1-14 5.3L4 15"/><path d="M20 4v5h-5"/><path d="M4 20v-5h5"/>',
    'progress': '<path d="M4 20V11"/><path d="M10 20V4"/><path d="M16 20v-9"/><path d="M21 20H3"/>',
    'settings': '<path d="M4 6h9"/><path d="M17 6h3"/><path d="M4 12h3"/><path d="M11 12h9"/><path d="M4 18h11"/><path d="M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
    'theme': '<circle cx="12" cy="12" r="8.5"/><path d="M12 3.5v17"/><path d="M12 8a4 4 0 0 1 0 8"/>',
    'mute': '<path d="M4 9.5v5h3.5l5 4v-13l-5 4z"/><path d="m16 9.5 5 5"/><path d="m21 9.5-5 5"/>',
    'sound': '<path d="M4 9.5v5h3.5l5 4v-13l-5 4z"/><path d="M16 9a4.2 4.2 0 0 1 0 6"/><path d="M18.8 6.2a8 8 0 0 1 0 11.6"/>',
    'lock': '<rect x="5" y="11" width="14" height="9.5" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
    'done': '<circle cx="12" cy="12" r="9"/><path d="m8 12.5 3 3 5-6"/>',
    'help': '<circle cx="12" cy="12" r="9"/><path d="M9.3 9.5a2.8 2.8 0 1 1 3.9 2.6c-.8.4-1.2 1-1.2 1.9"/><path d="M12 17h.01"/>',
    'admin': '<path d="M12 3 5 6v5.5c0 4.5 3 7.5 7 9.5 4-2 7-5 7-9.5V6z"/><circle cx="12" cy="10" r="2.2"/><path d="M8.5 16c.6-1.8 2-2.6 3.5-2.6s2.9.8 3.5 2.6"/>',
    'upload': '<path d="M12 16V4"/><path d="m7 9 5-5 5 5"/><path d="M4 20h16"/>',
    'image': '<rect x="3.5" y="4.5" width="17" height="15" rx="2.5"/><circle cx="9" cy="10" r="1.6"/><path d="m3.5 17 5-4.5 4 3.5 3-2.5 5 4.5"/>',
    'words': '<path d="M5 4.5h14a2 2 0 0 1 2 2V15a2 2 0 0 1-2 2h-7l-4.5 3.5V17H5a2 2 0 0 1-2-2V6.5a2 2 0 0 1 2-2z"/><path d="M7.5 9h9"/><path d="M7.5 12.5h5"/>',
}

STROKE_ATTRS = ('fill="none" stroke="currentColor" stroke-width="2" '
                'stroke-linecap="round" stroke-linejoin="round"')


def write_icons():
    sprite = ['<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" focusable="false" '
              'width="0" height="0" style="position:absolute">\n']
    for name in sorted(ICONS):
        body = ICONS[name]
        write('icons/%s.svg' % name,
              '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" %s '
              'aria-hidden="true" focusable="false">%s</svg>\n' % (STROKE_ATTRS, body))
        sprite.append('<symbol id="icon-%s" viewBox="0 0 24 24"><g %s>%s</g></symbol>\n'
                      % (name, STROKE_ATTRS, body))
    sprite.append('</svg>\n')
    write('sprite.svg', ''.join(sprite))


def main():
    for key in 'abc':
        write('logo-%s.svg' % key, logo_svg(key))
        write('logo-%s-dark.svg' % key, logo_svg(key, dark=True))
        write('mark-%s.svg' % key, mark_svg(key))
        write('wordmark-%s.svg' % key, wordmark_svg(key)[0])
        write('wordmark-%s-dark.svg' % key, wordmark_svg(key, dark=True)[0])
    pick = os.environ.get('BRAND_PICK', 'a')
    write('favicon.svg', favicon_svg(pick))
    # Full-bleed source for icon-192/512 and apple-touch-icon (rasterized by build-brand-assets.js)
    write('icon-maskable.svg', maskable_svg(pick))
    write_icons()
    print('wrote brand SVGs; pick =', pick)


if __name__ == '__main__':
    main()
