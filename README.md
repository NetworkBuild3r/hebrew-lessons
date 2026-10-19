hebrew v3 (greenfield, ADR-R1, ADR-0018). Publishes ghcr.io/networkbuild3r/hebrew. Never merges into main.

## Configuration

`cmd/hebrew` reads the process environment. `HEBREW_SESSION_SECRET` is the input key for the OAuth state cookie. The process derives a 32-byte HMAC key with HKDF-SHA256 (no salt, info `hebrew-lessons oauth-state v1`). Set the same value on every replica so a callback can be served by a different process than the one that started sign-in. When the variable is unset, the process generates a random key and logs a warning once; those state cookies do not survive a restart or another replica.
