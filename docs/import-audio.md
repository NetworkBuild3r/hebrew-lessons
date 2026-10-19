# Import legacy audio

One-shot command. It reads a verified export of the previous app's ElevenLabs clips and fills `audio_assets` plus `<DATA_DIR>/audio/<sha256>.mp3`. A later lookup plays those bytes instead of calling ElevenLabs again.

```
hebrew import-audio --from <export-dir-or-tgz> [--dry-run]
```

`DATA_DIR` is the same variable the server uses (default `/data`). The command opens `hebrew.sqlite` and runs migrations. `--dry-run` prints the same counts and writes the report and the manifest, and does not insert rows or mp3s.

`--from` is a directory, a `.tgz` / `.tar.gz`, or `-` for that gzip tar on stdin. A `.tgz` is read with `compress/gzip` and `archive/tar` and is not extracted. The archive root contains `clip-text-map.json` plus the lesson directories and `_audio/`. There is no `files/` wrapper. A single prefix such as `bundle/clip-text-map.json` is stripped. Schema `hebrew-clip-text-map/v3` requires `hashText` on every lesson and word. Schema `hebrew-clip-text-map/v2` is still accepted, and `hashText` there is optional. Any other schema is rejected.

Pack the tree from the directory that holds the map. The bundle name is `audio-import.tgz`. Lesson directories and `_audio/` sit at the archive root next to the map:

```
tar czf audio-import.tgz -C audio-export clip-text-map.json .
```

The first Phase 0 `audio.tgz` has only `files/` and no map. Do not import it.

Every mapped clip is checked before it is stored:

- When the map entry includes `sha256`, the mp3 bytes must hash to it (`hash-mismatch` otherwise). An entry without `sha256` is stored under the hash of its bytes.
- The digest is `sha256(voiceId + "\0" + model + "\0" + hashText.trim())` when `hashText` is present, and `sha256(voiceId + "\0" + model + "\0" + text.trim())` when it is absent. When `hashText` is present, `NormalizeText(hashText)` must equal `text` (`text-mismatch`, reason `nfc-differs`, otherwise), and the stored text is `text`. When it is absent, the stored text is `NormalizeText(text)`.
- A lesson clip is checked against `<lesson>/audio/manifest.json`: `{"version":1,"clips":{"audio/sentences/vocab-1.mp3":"<64-hex textHash>"}}`. Keys are relative to the lesson directory. The digest and the map's `textHash` must both equal `manifest.clips[key]`. A missing manifest, a missing key, or a manifest that cannot be parsed is a `text-mismatch`, and that clip is not stored.
- A word's file name must be the first 24 hex characters of that digest plus `.mp3` (`text-mismatch` otherwise). The word is also checked against `_audio/words/manifest.json` (`{"version":1,"entries":{"<voiceId>|<model>|<text>":{"text","file","voiceId","model",...}}}`). The digest must equal the hash of that entry's voice, model, and text, and `file` must be the clip. A missing manifest or a missing entry is a `text-mismatch`. The stored model remains `eleven_v4` or `eleven_v3` when that is the model the manifest names.

This is `textHash` / `cacheKey` from `server/audio.js` on `main`. The hash is the trimmed `hashText` when that field is set, and otherwise the trimmed `text`, not the normalized form. Unmapped entries are listed and not stored. The same bytes with two different texts are a `conflict`: both paths and texts are reported, and the second text does not replace the first. Each distinct sha256 that passes the checks is stored. The text index keeps one row per `(text, voice)`: `eleven_v4`, then `eleven_v3`, then `legacy`, then any other model, and within a model the lowest id. The other ids are `alias` in the summary and in `<DATA_DIR>/audio/import-manifest.json`.

Lookup preference, newest model first: `eleven_v4`, then `eleven_v3`, then `legacy`, then any other model, and within a model the newest `created_at`, then the lowest id.

## Kubernetes

Run this on staging `hebrew-next` only, after that deployment is live. Snapshot the PVC and `hebrew.sqlite` first. Keep the off-cluster export until the imported clips have been checked.

The image entrypoint is `/usr/local/bin/hebrew`. The image is distroless: no shell, no `tar`, no `mkdir`, no `rm`. `kubectl cp` runs `tar` inside the container, so copying the archive into this pod fails. Do not extract the archive.

Pipe `audio-import.tgz`. A stdin run spools the gzip to a temp file under `/data/audio` and removes it when the command returns, including on SIGTERM. Nothing is left on the volume except clips, the database, the report, and the manifest:

```
kubectl -n hebrew-next exec -i deploy/hebrew -- /usr/local/bin/hebrew import-audio --from - --dry-run < audio-import.tgz
kubectl -n hebrew-next exec -i deploy/hebrew -- /usr/local/bin/hebrew import-audio --from - < audio-import.tgz
```

If the `.tgz` is already a file on the data volume, point `--from` at it. `/data` exists in the image. `/data/import` does not, and the image cannot create it, so use `/data/audio-import.tgz` unless that directory is already there.

```
kubectl -n hebrew-next exec deploy/hebrew -- /usr/local/bin/hebrew import-audio --from /data/audio-import.tgz --dry-run
kubectl -n hebrew-next exec deploy/hebrew -- /usr/local/bin/hebrew import-audio --from /data/audio-import.tgz
```

Dry-run first. The command prints the summary, then exits non-zero when hash-mismatch, text-mismatch, conflict, or missing-file is above 0. Unmapped clips alone do not. Hebrew Dev's map v4 will add the remaining `eleven_v3` word clips, so imported and unmapped counts are not fixed here. A second run, after a successful import, inserts nothing and counts the stored clips as `duplicate`.

The unmapped, missing, mismatch, and conflict lists are written to `<DATA_DIR>/audio/import-unmapped.json`. The path to id to text list, including aliases, is `<DATA_DIR>/audio/import-manifest.json`. The importer does not delete `--from`. A stdin run leaves no archive. A file you placed on the volume has to be removed by hand from a pod that mounts the volume and has a shell. `kubectl exec deploy/hebrew -- rm` cannot work: this image has no `rm`.
