/**
 * Inline SVG icon set for דרך עברית (INIT-005/SPEC-006 starter).
 * Self-hosted, no CDN. All icons are 24x24 stroke icons, currentColor.
 * Usage: HebrewIcons.icon('today') -> svg string. Primary actions keep visible words.
 * Icon-only controls must add aria-label at call site.
 */
(function (root) {
    'use strict';

    var PATHS = {
        today: '<circle cx="12" cy="14" r="4"/><path d="M12 6v2M5.5 8.5l1.4 1.4M18.5 8.5 17.1 9.9M4 14h2M18 14h2M6 20h12"/>',
        lessons: '<path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v15H6.5A2.5 2.5 0 0 0 4 20.5Z"/><path d="M4 20.5A2.5 2.5 0 0 1 6.5 18H20"/>',
        review: '<path d="M4 12a8 8 0 0 1 14-5l2 2M20 12a8 8 0 0 1-14 5l-2-2"/><path d="M20 4v5h-5M4 20v-5h5"/>',
        progress: '<path d="M4 20V10M10 20V4M16 20v-8M21 20H3"/>',
        admin: '<path d="M12 3 5 6v5c0 5 3 8 7 10 4-2 7-5 7-10V6Z"/><path d="m9.5 12 2 2 3.5-4"/>',
        play: '<circle cx="12" cy="12" r="9"/><path d="m10 8.5 6 3.5-6 3.5Z"/>',
        sound: '<path d="M4 10v4h4l5 4V6Z"/><path d="M16.5 9a4.5 4.5 0 0 1 0 6M19 6.5a8 8 0 0 1 0 11"/>',
        check: '<circle cx="12" cy="12" r="9"/><path d="m8.5 12.5 2.5 2.5 4.5-5.5"/>',
        wrong: '<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/>',
        retry: '<path d="M4 12a8 8 0 1 1 2.3 5.7"/><path d="M4 18v-5h5"/>',
        back: '<path d="M19 12H5M11 6l-6 6 6 6"/>',
        next: '<path d="M5 12h14M13 6l6 6-6 6"/>',
        lock: '<rect x="5" y="11" width="14" height="9" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
        done: '<path d="m4.5 12.5 5 5 10-11"/>',
        help: '<circle cx="12" cy="12" r="9"/><path d="M9.5 9.5a2.5 2.5 0 1 1 3.6 2.2c-.8.4-1.1.9-1.1 1.8"/><circle cx="12" cy="17" r=".5"/>',
        mute: '<path d="M4 10v4h4l5 4V6Z"/><path d="m16 9 5 6M21 9l-5 6"/>',
        upload: '<path d="M12 16V4M7 9l5-5 5 5"/><path d="M4 20h16"/>',
        image: '<rect x="4" y="5" width="16" height="14" rx="2"/><circle cx="9" cy="10" r="1.5"/><path d="m4 17 5-5 4 4 3-3 4 4"/>',
        theme: '<path d="M12 3a9 9 0 1 0 0 18c1.5 0 2-1 1.3-2.2-.7-1.3 0-2.8 1.7-2.8H16a5 5 0 0 0 5-5c0-4.4-4-8-9-8Z"/>',
        spark: '<path d="M12 3v4M12 17v4M3 12h4M17 12h4M6 6l2.5 2.5M15.5 15.5 18 18M18 6l-2.5 2.5M8.5 15.5 6 18"/>'
    };

    function icon(name, size) {
        var inner = PATHS[name] || PATHS.spark;
        var s = size || 20;
        return '<svg class="hebrew-icon hebrew-icon-' + name + '" width="' + s + '" height="' + s
            + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"'
            + ' stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">'
            + inner + '</svg>';
    }

    function logoMark(size) {
        var s = size || 32;
        return '<svg class="hebrew-logo-mark" width="' + s + '" height="' + s + '" viewBox="0 0 512 512"'
            + ' aria-hidden="true" focusable="false">'
            + '<defs><linearGradient id="hwg" x1="0" y1="0" x2="1" y2="1">'
            + '<stop offset="0" stop-color="#1d4ed8"/><stop offset=".55" stop-color="#4338ca"/>'
            + '<stop offset="1" stop-color="#7c3aed"/></linearGradient></defs>'
            + '<rect width="512" height="512" rx="112" fill="url(#hwg)"/>'
            + '<text x="256" y="348" text-anchor="middle" font-family="Rubik, Arial, sans-serif"'
            + ' font-weight="700" font-size="280" fill="#ffffff">ע</text>'
            + '<circle cx="392" cy="128" r="36" fill="#fbbf24"/></svg>';
    }

    root.HebrewIcons = {
        PATHS: PATHS,
        icon: icon,
        logoMark: logoMark
    };
})(typeof window !== 'undefined' ? window : globalThis);
