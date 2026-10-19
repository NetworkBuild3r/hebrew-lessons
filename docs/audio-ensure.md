# audio-ensure: put the placement clips on a volume

INIT-015 / SPEC-026. The 206 placement clips (`internal/placement/audio-manifest.json`) are generated once, but each cluster volume needs its own copy. The image is distroless, so the command is a subcommand of the `hebrew` binary, and the manifest is embedded in it.

```
hebrew audio-ensure [--dry-run]
```

`DATA_DIR` is the same variable the server uses. The command opens `hebrew.sqlite`, runs migrations, and writes clips to `<DATA_DIR>/audio/`.

- **`--dry-run`** prints how many clips would be made and calls no provider. It does not need `ELEVENLABS_API_KEY`.
- **A real run** reads `ELEVENLABS_API_KEY`, `ELEVENLABS_VOICE_ID`, and `ELEVENLABS_MODEL` from the environment. For each text that has **no clip in the default voice** (`ELEVENLABS_VOICE_ID`), it makes one, ten texts at a time, with the same generator `generate_audio` uses.
- **It is idempotent.** A second run skips every stored clip and calls no provider.
- **Output is counts only**, on stdout: `audio-ensure: generated=206 skipped=0 failed=0 total=206`. A dry run prints `audio-ensure dry run: would_generate=206 skipped=0 total=206`. Progress (`batch 3 of 21`) goes to stderr. A failure prints its reason (at most three distinct ones), with the API key removed. The key and the voice ids are never printed. Exit status is 1 when any clip failed; run it again to retry only those.

Run the dry run first. Because the rule is "no clip in the default voice", a text that has a clip only in some other voice is made again in the default voice. The dry run count shows that before any credit is spent.

In the pod:

```
kubectl exec -n <ns> deploy/<hebrew> -- /usr/local/bin/hebrew audio-ensure --dry-run
kubectl exec -n <ns> deploy/<hebrew> -- /usr/local/bin/hebrew audio-ensure
```
