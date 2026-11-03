# k8s-pod-analyzer

Read container logs, count what went wrong, and rank the failures worth fixing
first. It works on a local file, on stdin, or straight from `kubectl logs`, and
it can write a self-contained HTML report you can attach to an incident ticket.

```bash
pod-analyzer analyze examples/incident.log --timeline
kubectl logs deploy/checkout-api --since=30m | pod-analyzer analyze -
pod-analyzer analyze examples/incident.log -o html > report.html
pod-analyzer kubectl -n payments --pod api-7d9f --since 30
```

[See a generated report](https://ankitavasudev.github.io/k8s-pod-analyzer/),
built from [`examples/incident.log`](examples/incident.log).

## Install

```bash
go install github.com/Ankitavasudev/k8s-pod-analyzer@v0.2.0
```

Go 1.21 or newer. The binary is called `pod-analyzer`. From a clone, run
`go build -o pod-analyzer .`

`pod-analyzer version` prints the version it was built from: the module version
when installed with `go install`, or the value compiled into the source when
built from a checkout.

## What it does

`analyze` reads the log one line at a time and classifies it:

| Level | Matched on (case-insensitive) |
|---|---|
| `ERROR` | `ERROR`, `FATAL`, `PANIC` |
| `WARN` | `WARN` or `WARNING` |
| `DEBUG` | `DEBUG` |
| `INFO` | everything else |

A leading `YYYY-MM-DD[T ]HH:MM:SS` is parsed as the timestamp. Lines without one
still count towards the totals but are left out of the timeline.

The result contains the total line count, a count per level, the ten most
frequent error messages (truncated to 120 characters), a per-minute histogram,
and — in `kubectl` mode — lines per pod and per namespace.

Before messages are counted, the timestamp and the level prefix are removed. So
`10:59:01 ERROR connection reset by peer` and `10:59:02 ERROR connection reset by
peer` are one entry with a count of two, instead of two entries with a count of
one each. Without that step "top errors" would just be the last ten lines of the
file.

Output is deterministic. Ties in the error ranking are broken by message, the
timeline is sorted chronologically, and the pod table is sorted by line count and
then by name, so two runs over the same input produce the same bytes.

## Output formats

```bash
pod-analyzer analyze app.log                  # text summary
pod-analyzer analyze app.log -o json          # machine readable
pod-analyzer analyze app.log -o csv           # three stacked CSV tables
pod-analyzer analyze app.log -o html          # self-contained report
pod-analyzer analyze app.log -l error         # only ERROR lines
pod-analyzer analyze app.log -t               # add the timeline histogram
```

The JSON output is `{"timestamp": ..., "summary": {...}}`, with the timeline as
an object keyed by minute.

The CSV output is three stacked tables — `metric,value`, `timestamp,count`,
`error_message,count` — separated by blank lines, with the timeline sorted
chronologically.

The HTML output is one file with inline CSS and no JavaScript or external
requests. It opens from disk, from a `file://` URL, or from any static host, and
log content is HTML-escaped before it is printed. That makes it safe to hand a
report to someone else, which is the point of the format.

## kubectl mode

```bash
pod-analyzer kubectl                       # every pod in the current namespace
pod-analyzer kubectl -n prod -p web-0      # one pod
pod-analyzer kubectl -n prod -c sidecar    # one container
pod-analyzer kubectl --since 15            # last 15 minutes (default 60)
```

Each call runs `kubectl logs --tail=1000`. A pod that fails to return logs is
skipped rather than aborting the run, so one crashlooping container does not stop
you from reading the rest. If `-p` is omitted, the pod list comes from
`kubectl get pods`.

## Layout

```
main.go        parsing, analysis, text/JSON/CSV/HTML rendering, cobra commands
main_test.go   tests: levels, timestamps, ranking, truncation, CSV and HTML output
examples/      sample.log and a 101-line incident log used by the README and CI
scripts/       build-demo.sh, which regenerates the published report
docs/          the report served by GitHub Pages
```

The only third-party dependency is `github.com/spf13/cobra`.

## Development

```bash
go test ./...
go vet ./...
gofmt -l .      # should print nothing
./scripts/build-demo.sh   # regenerate docs/index.html
```

## Licence

MIT — see [LICENSE](LICENSE).
