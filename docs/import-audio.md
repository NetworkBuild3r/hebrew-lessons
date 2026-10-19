# Import legacy audio

One-shot command. It reads a verified export of the previous app's ElevenLabs clips and fills `audio_assets` plus `<DATA_DIR>/audio/<sha256>.mp3`. A later lookup plays those bytes instead of calling ElevenLabs again.

```
hebrew import-audio --from <export-dir-or-tgz> [--dry-run]
```

`DATA_DIR` is the same variable the server uses (default `/data`). The command opens `hebrew.sqlite` and runs migrations. `--dry-run` prints the same counts and writes the report and the manifest, and does not insert rows or mp3s.

`--from` is a directory, a `.tgz` / `.tar.gz`, or `-` for that gzip tar on stdin. A `.tgz` is read with `compress/gzip` and `archive/tar` and is not extracted. The archive root contains `clip-text-map.json` (`schema` `hebrew-clip-text-map/v2`) plus the lesson directories and `_audio/`. There is no `files/` wrapper. A single prefix such as `bundle/clip-text-map.json` is stripped.

Pack the verified tree from the directory that holds the map:

```
tar czf audio-import.tgz -C audio-export clip-text-map.json .
```

The first Phase 0 `audio.tgz` has only `files/` and no map. Do not import it. The corrected bundle is the one whose sha256 starts `3d4f8191`.

Every mapped clip is checked before it is stored:

- `sha256` of the mp3 bytes must equal the map's `sha256` (`hash-mismatch` otherwise).
- `sha256(voiceId + "\0" + model + "\0" + text.trim())` must equal a lesson's `textHash`, and a word's file name must be the first 24 hex characters of that digest (`text-mismatch` otherwise).

This is `textHash` / `cacheKey` from `server/audio.js` on `main`. The hash is the trimmed text, not the normalized form. Unmapped entries (23 words absent from the words manifest, 24 `_archive` clips) are listed and not stored. The same bytes with two different texts are a `conflict`: both paths and texts are reported, and the second text does not replace the first. Each distinct sha256 that passes both checks is stored. The text index keeps one row per `(text, voice, model)`, the lowest id, which is what a lookup returns when `created_at` ties. The other ids are `alias` in the summary and in `<DATA_DIR>/audio/import-manifest.json`.

Nine lesson clips in the verified map are NFC, while the lesson manifest hashed a different combining-mark order. `text.trim()` of the map text does not equal `textHash`, so the importer refuses them. They are not stored.

Lookup preference, newest model first: `eleven_v4`, then `eleven_v3`, then `legacy`, then any other model, and within a model the newest `created_at`, then the lowest id.

## Kubernetes

Run this on staging `hebrew-next` only, after that deployment is live. Snapshot the PVC and `hebrew.sqlite` first. Keep the off-cluster export until the imported clips have been checked.

The image entrypoint is `/usr/local/bin/hebrew`. The image is distroless: no shell, no `tar`, no `mkdir`, no `rm`. `kubectl cp` runs `tar` inside the container, so copying the archive into this pod fails. Do not extract the archive.

Pipe the verified bundle. A stdin run spools the gzip to a temp file under `/data/audio` and removes it when the command returns, including on SIGTERM. Nothing is left on the volume except clips, the database, the report, and the manifest:

```
kubectl -n hebrew-next exec -i deploy/hebrew -- /usr/local/bin/hebrew import-audio --from - --dry-run < audio-import.tgz
kubectl -n hebrew-next exec -i deploy/hebrew -- /usr/local/bin/hebrew import-audio --from - < audio-import.tgz
```

If the `.tgz` is already a file on the data volume, point `--from` at it. `/data` exists in the image. `/data/import` does not, and the image cannot create it, so use `/data/audio.tgz` unless that directory is already there.

```
kubectl -n hebrew-next exec deploy/hebrew -- /usr/local/bin/hebrew import-audio --from /data/audio.tgz --dry-run
kubectl -n hebrew-next exec deploy/hebrew -- /usr/local/bin/hebrew import-audio --from /data/audio.tgz
```

On the verified bundle (548 mp3s), the first run is:

```
imported=492 duplicate=0 unmapped=47 hash-mismatch=0 text-mismatch=9 conflict=0 alias=33 missing-file=0
models: eleven_v4=484 eleven_v3=8
```

`unmapped=47` is 23 `word-not-in-manifest` plus 24 `archive`. `alias=33` is the extra stored assets in 26 groups that share text, voice, and model but not bytes (27 groups in the map; one group loses its second clip to `text-mismatch`). `text-mismatch=9` is the combining-mark set, by lesson:

| lesson | clips |
| --- | --- |
| he-said-that-retell | audio/story/line-1.mp3 |
| israel-trip-kshe | audio/sentences/friend-2.mp3, audio/sentences/vocab-4.mp3 |
| next-weekend-going-to | audio/sentences/vocab-8.mp3 |
| shai-date-indirect-speech | audio/sentences/friend-1.mp3, friend-2.mp3, friend-3.mp3, friend-4.mp3 |
| steakhouse-et | audio/sentences/vocab-4.mp3 |

The 55 story lines that the first map truncated (`direct` / `reported` / `strong` dropped) are in this bundle with the full text. Per lesson: date-night-maadif-infinitive 8, he-said-that-retell 6, israel-trip-kshe 4, last-weekend-past-ti 1, my-morning-one-verb-two-verbs 3, plane-small-talk-yesh-li 15, say-it-again-efshar 8, shai-date-indirect-speech 6, steakhouse-et 4. Those 55 verify and are part of the 492.

A second run, after the first one stored the clips, prints `imported=0 duplicate=492` with the same unmapped, mismatch, conflict, and alias counts.

The unmapped, missing, mismatch, and conflict lists are written to `<DATA_DIR>/audio/import-unmapped.json`. The path to id to text list, including aliases, is `<DATA_DIR>/audio/import-manifest.json`. The importer does not delete `--from`. A stdin run leaves no archive. A file you placed on the volume has to be removed by hand from a pod that mounts the volume and has a shell. `kubectl exec deploy/hebrew -- rm` cannot work: this image has no `rm`.
