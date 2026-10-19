-- Tanakh verses. One row per book, chapter, and verse.
-- Hebrew is pointed NFC. he_plain is units.Plain of that Hebrew:
-- niqqud, meteg, rafe, cantillation, and upper dots removed; shin and
-- sin dots kept. English is JPS 1917. The importer refuses a license
-- other than Public Domain, so the license column cannot store anything else.
-- PRIMARY KEY (book, chapter, verse) is the lookup index for that triple.

CREATE TABLE tanakh_verses (
    book TEXT NOT NULL CHECK (
        length(book) BETWEEN 1 AND 40
        AND book NOT GLOB '*[^a-z0-9-]*'
    ), -- Canonical slug: genesis, i-samuel, song-of-songs.
    chapter INTEGER NOT NULL CHECK (chapter >= 1), -- 1-based chapter.
    verse INTEGER NOT NULL CHECK (verse >= 1), -- 1-based verse.
    he TEXT NOT NULL CHECK (length(he) BETWEEN 1 AND 2000), -- Pointed Hebrew, NFC, whitespace collapsed.
    he_plain TEXT NOT NULL CHECK (length(he_plain) BETWEEN 1 AND 2000), -- Consonant skeleton of he. Maqaf and punctuation stay; lookup tokenizes them.
    en TEXT NOT NULL CHECK (length(en) BETWEEN 1 AND 2000), -- English translation, JPS 1917, NFC.
    source_version TEXT NOT NULL CHECK (length(source_version) BETWEEN 1 AND 400), -- Edition titles this row was imported from.
    license TEXT NOT NULL CHECK (license = 'Public Domain'), -- Only Public Domain text is stored.
    PRIMARY KEY (book, chapter, verse)
);

-- Provenance and the import marker. content_hash is the sha256 of the
-- sorted corpus files. A later import with the same hash writes nothing.
CREATE TABLE tanakh_meta (
    key TEXT PRIMARY KEY CHECK (length(key) BETWEEN 1 AND 80), -- Marker name, for example content_hash.
    value TEXT NOT NULL CHECK (length(value) <= 4000) -- Marker value.
);
