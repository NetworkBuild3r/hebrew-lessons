# Rebuild daily history

Operator command for ADR-0028 D3 / ADR-0030 D4 / INIT-035 SPEC-004. Rewrites `daily_snapshots` and `daily_totals` as of each day under the current rules (`rule_version = 3`). It never touches regression events, attempts, reviews, or cards.

Dry run is the default and opens the database with `PRAGMA query_only`. `--apply` writes one transaction per day and takes the same `daily.lock` as the scheduled job.

The image is distroless (no shell). Find the pod by name and run the binary directly.

## Order

### 1. Backup first

```sh
POD=$(kubectl -n hebrew get pod -o name | head -n1)
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew backup --to /data/backups/hebrew-before-history-rebuild.sqlite
```

`DATA_DIR` defaults to `/data` in the image (`ENV DATA_DIR=/data`), so the live database is `/data/hebrew.sqlite` unless `--db` is set. Record `bytes=` and `sha256=` from the backup output.

### 2. Dry run

```sh
POD=$(kubectl -n hebrew get pod -o name | head -n1)
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew history rebuild --learner 1
```

Or with an explicit path:

```sh
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew history rebuild --learner 1 --db /data/hebrew.sqlite
```

Output is a per-day table: `known_any_skill old→new`, and per skill `known_solid old→new` and `est old→new`. It writes nothing. Show the table to Brian before apply (SPEC-009).

### 3. Apply

Only after backup and dry-run review:

```sh
POD=$(kubectl -n hebrew get pod -o name | head -n1)
kubectl -n hebrew exec "$POD" -c hebrew -- /usr/local/bin/hebrew history rebuild --learner 1 --apply
```

Prints the same table plus `applied N days`. A second `--apply` is idempotent: the table matches and regression event counts do not change.

If the daily job holds the lock, the command exits 1 and names the holder. Wait for the job to finish, or retry after the 30-minute lock TTL.

## Flags

| Flag | Meaning |
| --- | --- |
| `--learner N` | Required. Positive learner id. |
| `--from YYYY-MM-DD` | First day. Default: first snapshot day, else first attempt day in the learner's zone. |
| `--to YYYY-MM-DD` | Last day. Default: yesterday in the learner's zone. |
| `--db PATH` | Database file. Default: `$DATA_DIR/hebrew.sqlite` (`/data/hebrew.sqlite` in the pod). |
| `--apply` | Write rebuilt rows. Without it, dry-run only. |

Unknown learner or a learner with no evidence exits 1 and names the reason.
