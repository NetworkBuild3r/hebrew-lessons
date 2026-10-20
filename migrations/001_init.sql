-- Clean start for the v2 app. Nothing is imported from the previous database.
-- The runner creates schema_migrations; this file is the domain model only.

-- A person who signs in with Google.
CREATE TABLE users (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    email TEXT NOT NULL UNIQUE, -- Google account email, stored lowercase.
    google_sub TEXT NOT NULL UNIQUE, -- Google subject (sub). One Google account maps to one user.
    role TEXT NOT NULL CHECK (role IN ('learner', 'admin')), -- learner uses the course; admin may author and publish.
    created_at TEXT NOT NULL -- UTC RFC3339 time the row was inserted, from the process clock.
);

-- A browser session. The cookie carries the raw id; the database stores only its hash.
CREATE TABLE sessions (
    id_hash TEXT PRIMARY KEY, -- Digest of the session id. The raw id is never stored.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Owner. Deleting the user deletes the session.
    csrf TEXT NOT NULL, -- Secret checked with the Origin header on mutating requests.
    expires_at TEXT NOT NULL -- UTC RFC3339 expiry. Requests after this time are signed out.
);

CREATE INDEX sessions_user ON sessions (user_id);

-- One generated mp3. The file on disk is <DATA_DIR>/audio/<id>.mp3.
CREATE TABLE audio_assets (
    id TEXT PRIMARY KEY CHECK (
        length(id) = 64 AND lower(id) = id AND id NOT GLOB '*[^0-9a-f]*'
    ), -- Lowercase hex sha256 of the mp3 bytes. The bytes themselves are not stored.
    text_nfc TEXT NOT NULL, -- Exact pointed Hebrew, NFC, single spaces, that this clip speaks.
    voice TEXT NOT NULL, -- ElevenLabs voice id used to synthesize the clip.
    model TEXT NOT NULL, -- ElevenLabs model id used to synthesize the clip.
    bytes INTEGER NOT NULL CHECK (bytes >= 0), -- File size in bytes, not the payload.
    created_at TEXT NOT NULL -- UTC RFC3339 time the clip was stored.
);

-- A dictionary entry. text_nfc is the identity of the word across lessons and audio.
CREATE TABLE lexemes (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    text_nfc TEXT NOT NULL UNIQUE, -- Pointed Hebrew in NFC with internal whitespace collapsed to one space. Writers normalize; the database does not.
    bare TEXT NOT NULL DEFAULT '', -- Unpointed spelling used for search. Empty when it has not been filled in.
    gloss_en TEXT NOT NULL DEFAULT '', -- Short English gloss.
    pos TEXT NOT NULL DEFAULT '', -- Part of speech, free text until the lexicon package constrains it.
    root TEXT NOT NULL DEFAULT '', -- Consonantal root. Empty for words that have none.
    binyan TEXT NOT NULL DEFAULT '', -- Verb pattern. Empty for non-verbs and unknown verbs.
    notes TEXT NOT NULL DEFAULT '', -- Author notes. Not shown to learners.
    audio_id TEXT REFERENCES audio_assets (id) ON DELETE SET NULL, -- Clip for this exact text. Null until audio exists. Deleting the clip keeps the lexeme.
    verified INTEGER NOT NULL DEFAULT 0 CHECK (verified IN (0, 1)) -- 1 once a person has confirmed the entry.
);

CREATE INDEX lexemes_audio ON lexemes (audio_id);

-- A course unit. doc is the authoring document; the other columns are what lists and filters read.
CREATE TABLE units (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    position INTEGER NOT NULL CHECK (position >= 0), -- Order in the course. Not unique, so a reorder can temporarily share a slot.
    title TEXT NOT NULL, -- Learner-facing title. Copied from doc for lists.
    situation TEXT NOT NULL, -- The scene the unit is set in.
    concept TEXT NOT NULL, -- The one idea the unit teaches.
    status TEXT NOT NULL CHECK (status IN ('draft', 'published')), -- draft is editable; published is what learners open.
    rev INTEGER NOT NULL CHECK (rev >= 1), -- Current revision number. Matches a unit_revisions row.
    doc TEXT NOT NULL CHECK (json_valid(doc)) -- Full unit JSON document.
);

CREATE INDEX units_status_position ON units (status, position);

-- One saved copy of a unit document. put_unit inserts a row; it does not overwrite history.
CREATE TABLE unit_revisions (
    unit_id INTEGER NOT NULL REFERENCES units (id) ON DELETE CASCADE, -- Unit this copy belongs to. Deleting the unit deletes its history.
    rev INTEGER NOT NULL CHECK (rev >= 1), -- Revision number, unique per unit.
    doc TEXT NOT NULL CHECK (json_valid(doc)), -- Unit JSON at this revision.
    actor TEXT NOT NULL, -- Who wrote it: a user id or an agent name.
    created_at TEXT NOT NULL, -- UTC RFC3339 time this revision was saved.
    PRIMARY KEY (unit_id, rev)
);

-- Words a lesson uses. Derived when a unit is saved. Feeds the coverage and recycling rules.
CREATE TABLE lesson_items (
    unit_id INTEGER NOT NULL REFERENCES units (id) ON DELETE CASCADE, -- Unit whose lesson lists this word. Deleting the unit deletes the rows.
    lesson_ix INTEGER NOT NULL CHECK (lesson_ix >= 0), -- Zero-based index into the unit's lessons array.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word. A lexeme that is still taught cannot be deleted.
    role TEXT NOT NULL CHECK (role IN ('new', 'recycle')), -- new introduces the word; recycle practises one from an earlier lesson.
    skill TEXT NOT NULL, -- Skill this row practises: listen_pick, recall, produce, cloze, pattern, story, or shadow.
    PRIMARY KEY (unit_id, lesson_ix, lexeme_id)
);

CREATE INDEX lesson_items_lexeme ON lesson_items (lexeme_id);

-- One FSRS card: a learner, a lexeme, and a direction.
CREATE TABLE cards (
    id INTEGER PRIMARY KEY, -- Surrogate key. review_log rows point here.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their cards.
    lexeme_id INTEGER NOT NULL REFERENCES lexemes (id) ON DELETE RESTRICT, -- Word. Deleting a lexeme that still has cards is refused.
    dir TEXT NOT NULL CHECK (dir IN ('he_en', 'en_he')), -- he_en is receptive (Hebrew to English); en_he is productive.
    state TEXT NOT NULL CHECK (state IN ('new', 'learning', 'review', 'relearning')), -- FSRS state. Maps to go-fsrs New, Learning, Review, Relearning.
    due TEXT NOT NULL, -- UTC RFC3339 time the card is due.
    stability REAL NOT NULL CHECK (stability >= 0), -- FSRS stability, in days.
    difficulty REAL NOT NULL CHECK (difficulty >= 0), -- FSRS difficulty.
    reps INTEGER NOT NULL CHECK (reps >= 0), -- How many times the card has been reviewed.
    lapses INTEGER NOT NULL CHECK (lapses >= 0), -- How many times the card has been forgotten.
    last_review TEXT, -- UTC RFC3339 time of the previous review. Null until the first review.
    suspended INTEGER NOT NULL DEFAULT 0 CHECK (suspended IN (0, 1)), -- 1 keeps the card out of the queue without deleting its history.
    UNIQUE (user_id, lexeme_id, dir)
);

-- Due queue: one learner, soonest due first. Suspended cards stay in the index and are filtered by the query.
CREATE INDEX cards_user_due ON cards (user_id, due);

-- One review of one card. Append-only so FSRS can be optimized later.
CREATE TABLE review_log (
    id INTEGER PRIMARY KEY, -- Surrogate key.
    card_id INTEGER NOT NULL REFERENCES cards (id) ON DELETE CASCADE, -- Card that was reviewed. Deleting the card deletes its log.
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 4), -- FSRS rating: 1 Again, 2 Hard, 3 Good, 4 Easy.
    reviewed_at TEXT NOT NULL, -- UTC RFC3339 time of the review.
    elapsed_days INTEGER NOT NULL CHECK (elapsed_days >= 0), -- Days since the previous review, as FSRS recorded them.
    scheduled_days INTEGER NOT NULL CHECK (scheduled_days >= 0) -- Days this rating scheduled until the next due time.
);

CREATE INDEX review_log_card ON review_log (card_id);

-- A learner's pass through one lesson of one unit.
CREATE TABLE lesson_runs (
    id INTEGER PRIMARY KEY, -- Surrogate key. A learner may run the same lesson more than once.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- Learner. Deleting the user deletes their runs.
    unit_id INTEGER NOT NULL REFERENCES units (id) ON DELETE CASCADE, -- Unit. Deleting the unit deletes its runs.
    lesson_ix INTEGER NOT NULL CHECK (lesson_ix >= 0), -- Zero-based index into the unit's lessons array.
    started_at TEXT NOT NULL, -- UTC RFC3339 time the run started.
    finished_at TEXT, -- UTC RFC3339 time the run finished. Null while it is in progress.
    score REAL CHECK (score IS NULL OR (score >= 0 AND score <= 1)) -- Fraction correct. Null until the run finishes.
);

CREATE INDEX lesson_runs_user_unit ON lesson_runs (user_id, unit_id, lesson_ix);

-- A dynamically registered MCP OAuth client. Secrets are stored only as digests.
CREATE TABLE oauth_clients (
    client_id TEXT PRIMARY KEY, -- Opaque id returned to the client.
    client_secret_hash TEXT, -- Digest of the client secret. Null for a public client (method none). The raw secret is never stored.
    client_name TEXT NOT NULL, -- Display name from registration.
    redirect_uris TEXT NOT NULL CHECK (json_valid(redirect_uris)), -- JSON array of allowed redirect URIs.
    token_endpoint_auth_method TEXT NOT NULL CHECK (
        token_endpoint_auth_method IN ('none', 'client_secret_post', 'client_secret_basic')
    ), -- How the token endpoint authenticates this client.
    grant_types TEXT NOT NULL CHECK (json_valid(grant_types)), -- JSON array of grant types the client may use.
    response_types TEXT NOT NULL CHECK (json_valid(response_types)), -- JSON array of response types. This server issues code.
    scope TEXT NOT NULL DEFAULT '', -- Space-delimited scopes registered for the client.
    created_at TEXT NOT NULL -- UTC RFC3339 registration time.
);

-- An in-flight authorization request, before a code is issued.
CREATE TABLE oauth_requests (
    id TEXT PRIMARY KEY, -- Opaque request id, also the state we send to Google.
    client_id TEXT NOT NULL REFERENCES oauth_clients (client_id) ON DELETE CASCADE, -- Client that started the request. Deleting the client deletes it.
    redirect_uri TEXT, -- Redirect URI for this request. Null only when the client has not sent one yet.
    state TEXT, -- Client's state parameter, returned unchanged on the redirect.
    code_challenge TEXT NOT NULL, -- PKCE S256 challenge. The verifier is not stored.
    code_challenge_method TEXT NOT NULL CHECK (code_challenge_method = 'S256'), -- Only S256 is accepted.
    scope TEXT, -- Scope requested. Null when the client omitted it.
    resource TEXT, -- RFC 8707 resource indicator. Null when the client omitted it.
    expires_at TEXT NOT NULL -- UTC RFC3339 time this request can no longer be completed.
);

CREATE INDEX oauth_requests_client ON oauth_requests (client_id);

-- An authorization code. The raw code is shown once; only its digest is stored.
CREATE TABLE oauth_codes (
    code_hash TEXT PRIMARY KEY, -- Digest of the authorization code. The raw code is never stored.
    client_id TEXT NOT NULL REFERENCES oauth_clients (client_id) ON DELETE CASCADE, -- Client the code was issued to.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- User who approved the request.
    redirect_uri TEXT, -- Redirect URI bound to the code. The token request must match it when it sends one.
    code_challenge TEXT NOT NULL, -- PKCE S256 challenge the token request's verifier must satisfy.
    scope TEXT, -- Scope granted. Null when the request had none.
    resource TEXT, -- Resource bound to the code. Null when the request had none.
    expires_at TEXT NOT NULL, -- UTC RFC3339 expiry.
    used_at TEXT -- UTC RFC3339 time the code was exchanged. Null until then. A second use is rejected.
);

CREATE INDEX oauth_codes_client ON oauth_codes (client_id);

-- An access or refresh token. The raw token is shown once; only its digest is stored.
CREATE TABLE oauth_tokens (
    token_hash TEXT PRIMARY KEY, -- Digest of the access or refresh token. The raw token is never stored.
    kind TEXT NOT NULL CHECK (kind IN ('access', 'refresh')), -- Which token this digest is.
    client_id TEXT NOT NULL REFERENCES oauth_clients (client_id) ON DELETE CASCADE, -- Client the token was issued to.
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE, -- User the token acts as.
    scope TEXT, -- Scope granted. Null when the grant had none.
    resource TEXT, -- Resource bound to the token. Null when the grant had none.
    family_id TEXT NOT NULL, -- Refresh-token family. Rotation revokes every row in the family.
    expires_at TEXT NOT NULL, -- UTC RFC3339 expiry.
    revoked_at TEXT, -- UTC RFC3339 revocation time. Null while the token is live.
    created_at TEXT NOT NULL -- UTC RFC3339 time the token was issued.
);

CREATE INDEX oauth_tokens_client ON oauth_tokens (client_id);
CREATE INDEX oauth_tokens_family ON oauth_tokens (family_id);
