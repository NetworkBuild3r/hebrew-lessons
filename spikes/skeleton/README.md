# Walking skeleton

One beginner unit, rendered locally. Nothing here is served by the production server. No install step and no network call.

Provenance: INIT-003/SPEC-009.

## Run

From the repository root:

```text
node spikes/skeleton/validate.js
node --test spikes/skeleton/test/
node spikes/skeleton/server.js
```

The server listens on `http://127.0.0.1:8099/` and prints that address. Open it in a browser. Stop it with Ctrl+C.

Screenshots from a local Chrome, if you want to compare them with the page: `spikes/skeleton/shots/unit-1280.png` and `spikes/skeleton/shots/unit-360.png`.

`validate.js` checks the four ADR-0003 samples and rejects a script tag, a missing alt, oversized text, a bad media id, an unknown media id, and a missing required field.

The page is Today, the unit overview, one introduction (picture and an audio placeholder), a recall step, and the checkpoint. The checkpoint lists the words in that lesson's `recycles` array.

## Edit one part

The form at the bottom of the page and the command both call `ops.applyBody`.

```text
node spikes/skeleton/cli.js spikes/skeleton/test/fixtures/set-image.json
```

That prints the updated plan as JSON. It does not write the sample file. A stale `expectedRevision` prints a conflict and exits 1.

On the page, change the media id or the alt, then save. A revision that is not the current one is a conflict. The picture id is a catalog key (`coffee-cup`, `water-glass`, `milk`, `sugar`), not a URL.

## Limits

- Node built-ins only. No packages are added.
- The audio control is a placeholder. The sample plan does not include clip paths, and this spike does not invent any.
- Pictures resolve through the spike catalog to `public/media/placeholder-coffee.png`.
