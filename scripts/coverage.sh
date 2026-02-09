#!/bin/bash
echo "Running tests with coverage..."
go test -coverprofile=coverage.out ./pkg/... ./cmd/... ./tests/integration/...
go tool cover -func=coverage.out
echo "Generate HTML report: go tool cover -html=coverage.out"
