# K8s Pod Analyzer

> Kubernetes pod log analyzer with JSON/CSV output, filter flags, and report generation.

[![CI](https://github.com/Ankitavasudev/k8s-pod-analyzer/actions/workflows/ci.yml/badge.svg)](https://github.com/Ankitavasudev/k8s-pod-analyzer/actions)
[![Go 1.21+](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Features

- **Log Analysis** - Parse pod logs, extract errors and warnings
- **JSON/CSV Output** - Machine-readable export formats
- **Filter Flags** - Filter by status, restart count, errors, namespace
- **Report Generator** - HTML and text report generation
- **Real-time Monitoring** - Watch mode for live log streaming

## Quick Start

```bash
# Install
git clone https://github.com/Ankitavasudev/k8s-pod-analyzer.git
cd k8s-pod-analyzer
go build -o pod-analyzer .

# Analyze all pods
./pod-analyzer

# Filter by namespace
./pod-analyzer -n default

# Filter by status
./pod-analyzer --status CrashLoopBackOff

# Filter by restart count
./pod-analyzer --restarts 5

# Show only error logs
./pod-analyzer --errors

# JSON output
./pod-analyzer --format json

# CSV output
./pod-analyzer --format csv

# Generate report
./pod-analyzer --report report.html
```

## Filter Options

| Flag | Description | Example |
|------|-------------|---------|
| `-n` | Namespace filter | `-n default` |
| `--status` | Pod status filter | `--status Running` |
| `--restarts` | Min restart count | `--restarts 3` |
| `--errors` | Show only error logs | `--errors` |
| `--format` | Output format | `--format json` |
| `--report` | Generate report | `--report report.html` |

## Output Example

```json
{
  "pod_name": "nginx-abc123",
  "namespace": "default",
  "status": "Running",
  "restart_count": 0,
  "log_errors": [
    {
      "timestamp": "2026-08-09T12:00:00Z",
      "level": "error",
      "message": "Connection refused"
    }
  ]
}
```

## Architecture

```
k8s-pod-analyzer/
├── main.go         # CLI entry point, flags, analysis logic
├── output.go       # JSON/CSV output formatting
├── filters.go      # Filter functions
├── output_test.go  # Output tests
└── main_test.go    # Main logic tests
```

## Tech Stack

- **Go 1.21+** - Core language
- **kubectl** - Kubernetes API access
- **encoding/json** - JSON output
- **encoding/csv** - CSV output
- **testing** - Unit tests

## License

MIT License - see [LICENSE](LICENSE) for details.