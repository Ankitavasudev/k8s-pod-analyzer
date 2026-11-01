# k8s-pod-analyzer

Count errors and warnings in container logs, rank the most frequent failure
messages, and bucket them by minute. Works on a local log file, stdin, or
straight from `kubectl logs`.

```bash
go build -o pod-analyzer .

./pod-analyzer analyze examples/sample.log --timeline
cat examples/sample.log | ./pod-analyzer analyze -
./pod-analyzer kubectl -n payments --pod api-7d9f --since 30
```

## What it does

`analyze` reads one line at a time and classifies it:

| Level | Matched on (case-insensitive) |
|---|---|
| `ERROR` | `ERROR`, `FATAL`, `PANIC` |
| `WARN` | `WARN` |
| `DEBUG` | `DEBUG` |
| `INFO` | everything else |

A leading `YYYY-MM-DD[T ]HH:MM:SS` is parsed as the timestamp; lines without one
still count towards the totals but are left out of the timeline.

The result contains total lines, a count per level, the ten most frequent error
messages (truncated to 120 characters), a per-minute histogram, and â€” in
`kubectl` mode â€” lines per pod and per namespace.

## Output formats

```bash
./pod-analyzer analyze app.log                  # text summary
./pod-analyzer analyze app.log -o json          # {"timestamp": ..., "summary": {...}}
./pod-analyzer analyze app.log -o csv           # three CSV tables
./pod-analyzer analyze app.log -l error         # only ERROR lines
./pod-analyzer analyze app.log -t               # add the timeline histogram
```

The CSV output is three stacked tables â€” `metric,value`, `timestamp,count`,
`error_message,count` â€” with the timeline sorted chronologically so repeated
runs produce identical bytes.

## kubectl mode

```bash
./pod-analyzer kubectl                       # every pod in the current namespace
./pod-analyzer kubectl -n prod -p web-0      # one pod
./pod-analyzer kubectl -n prod -c sidecar    # one container
./pod-analyzer kubectl --since 15            # last 15 minutes (default 60)
```

Each call runs `kubectl logs --tail=1000`; a pod that fails to return logs is
skipped rather than aborting the run. If `-p` is omitted, the pod list comes
from `kubectl get pods`.

## Layout

```
main.go       parsing, analysis, text/JSON/CSV rendering, cobra commands
main_test.go  14 tests: levels, timestamps, ranking, truncation, CSV stability
examples/     a small log file used by the README and CI
```

No third-party code except cobra.

## Development

```bash
go test ./...
go vet ./...
gofmt -l .      # should print nothing
```

## Licence

MIT â€” see [LICENSE](LICENSE).
