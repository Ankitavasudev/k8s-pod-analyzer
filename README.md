# k8s-pod-analyzer

Kubernetes pod log analyzer CLI tool. Parse logs, classify errors, visualize timelines.

## Install
`ash
go install github.com/Ankitavasudev/k8s-pod-analyzer@latest
`

## Usage
`ash
k8s-pod-analyzer analyze pod-logs.txt
kubectl logs my-pod | k8s-pod-analyzer analyze -
`

## Features
- Error/warning/info classification
- Timeline visualization
- Top error messages
- Pipe support (stdin)
