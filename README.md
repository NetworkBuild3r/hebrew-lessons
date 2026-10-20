hebrew v3 (greenfield, ADR-R1, ADR-0018). Publishes ghcr.io/networkbuild3r/hebrew. Never merges into main.

## Authoring

A `recall`, `produce`, or `cloze` whose line has two or more words that are in the lexicon sets `target` to the exact pointed word it grades. That spelling must be a lexicon entry. The unpointed spelling does not count. A line with fewer lexicon words may omit `target`. On `listen_pick`, `target` is optional: if `target` is set, only that word gets credit; if not, nothing is credited and the step is logged as an exposure. A new `validate_unit`, `put_unit`, or `publish_unit` blocks a missing target on `recall`, `produce`, or `cloze`, and so does publishing an older revision. The current published revision still serves, and checking that same revision again reports a warning.

Passage coverage counts the brief's words, the learner's known words, and this unit's `newWords` when each is an exact pointed lexicon entry. The intro and the minimum uses run on the brief's new words and on every `newWord` that coverage credited, including one outside the brief. A word in none of those sets is unknown.

## Configuration

`cmd/hebrew` reads the process environment. `HEBREW_SESSION_SECRET` is the input key for the OAuth state cookie. The process derives a 32-byte HMAC key with HKDF-SHA256 (no salt, info `hebrew-lessons oauth-state v1`). Set the same value on every replica so a callback can be served by a different process than the one that started sign-in. When the variable is unset, the process generates a random key and logs a warning once; those state cookies do not survive a restart or another replica.

`MCP_AUTH_TOKEN` is the admin bearer for the authoring MCP. A matching request is an admin and may call every tool. An empty value disables that bearer. The process still starts. The comparison is constant time.

`MCP_READONLY_TOKEN` is the optional smoke bearer, trimmed, and off when empty. A matching request is a learner bound to `MCP_READONLY_LEARNER_ID`, which must be a positive integer. That caller may use `get_context`, `lookup`, `get_learner_words`, `validate_unit`, and `get_unit`. `get_context` and `get_learner_words` succeed only for that learner id. The bearer is compared in constant time, the same way as `MCP_AUTH_TOKEN`. The process does not create a users row. Point `MCP_READONLY_LEARNER_ID` at a dedicated smoke learner so the smoke does not read a real student. `get_context` still succeeds when that learner has no placement row and reports an empty placement. Both bearers work whether OAuth is on or off. Startup logs a fixed reason and refuses to start when the token is set and the learner id is missing, not an integer, or not positive; when the token equals `MCP_AUTH_TOKEN`; or when the token is shorter than 32 characters. The token value is not logged.
