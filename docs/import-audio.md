# Import legacy audio

One-shot command. It reads an export of the previous app's ElevenLabs clips and fills `audio_assets` plus `<DATA_DIR>/audio/<sha256>.mp3`. A later lookup plays those bytes instead of calling ElevenLabs again.

```
hebrew import-audio --from <export-dir-or-tgz> [--dry-run]
```

`DATA_DIR` is the same variable the server uses (default `/data`). The command opens `hebrew.sqlite` and runs migrations. `--dry-run` prints the same counts and writes the unmapped report, and does not insert rows or mp3s.

`--from` is either a directory or a `.tgz` / `.tar.gz`. A `.tgz` is read with `compress/gzip` and `archive/tar` and is not extracted. `--from -` reads that same gzip tar from stdin. The export root contains `clip-text-map.json` and `files/`.

Word clips are `files/_audio/words/<file>.mp3` (`voice` and `model` from the map). Lesson clips are `files/<lesson>/<clip>` (`voice` empty, `model` `legacy`). `unmapped_word_files` and any path containing `/_archive/` are skipped. The same text may have several rows (`eleven_v4` and `eleven_v3`). The same bytes under two names are one row. Running the command again inserts nothing.

Lookup preference, newest model first: `eleven_v4`, then `eleven_v3`, then `legacy`, then any other model, and within a model the newest `created_at`.

## Kubernetes

The image entrypoint is `/usr/local/bin/hebrew`. The image is distroless: no shell, no `tar`, no `mkdir`, no `rm`. `kubectl cp` runs `tar` inside the container, so copying the archive into this pod fails. Do not extract the archive.

Pipe it. Nothing is left on the volume except clips, the database, and the report:

```
kubectl -n hebrew-next exec -i deploy/hebrew -- /usr/local/bin/hebrew import-audio --from - --dry-run < audio.tgz
kubectl -n hebrew-next exec -i deploy/hebrew -- /usr/local/bin/hebrew import-audio --from - < audio.tgz
```

If the `.tgz` is already a file on the data volume, point `--from` at it. `/data` exists in the image. `/data/import` does not, and the image cannot create it, so use `/data/audio.tgz` unless that directory is already there.

```
kubectl -n hebrew-next exec deploy/hebrew -- /usr/local/bin/hebrew import-audio --from /data/audio.tgz --dry-run
kubectl -n hebrew-next exec deploy/hebrew -- /usr/local/bin/hebrew import-audio --from /data/audio.tgz
```

On the real export (548 mp3s): 104 word clips and 397 lesson clips are mapped. `skipped-unmapped` is 23 and `skipped-archive` is 24 (the `_archive` mp3s under `anal-sex-vocab`). `missing-file` is 0. `imported` is the number of distinct sha256 values, at most 501; the map has no byte hashes, so a first run's `duplicate` is however many files repeat an earlier file. `models` counts files read, not unique hashes: `eleven_v4=96 eleven_v3=8 legacy=397`.

A second run, after the first one stored the clips, prints `imported=0` and `duplicate=501`. The model line stays the same.

The unmapped and missing paths are printed and written to `<DATA_DIR>/audio/import-unmapped.json` (`/data/audio/import-unmapped.json` in the image). The importer does not delete `--from`. A stdin run leaves no archive. A file you placed on the volume has to be removed by hand from a pod that mounts the volume and has a shell. `kubectl exec deploy/hebrew -- rm` cannot work: this image has no `rm`.
