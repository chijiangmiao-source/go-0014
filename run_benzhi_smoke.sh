#!/usr/bin/env bash
set -euo pipefail

tmpdir=".benzhi-smoke.$$"
mkdir -p "$tmpdir"
pid=""
cleanup() {
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT

probe="${tmpdir}/probe.go"
cat >"$probe" <<'GO'
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func request(method, url, body, idem string) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return "", err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return string(data), fmt.Errorf("status %d", resp.StatusCode)
	}
	return string(data), nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: probe get URL | post URL BODY IDEMPOTENCY_KEY | wait-ready URL ATTEMPTS SLEEP_MS")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "get":
		body, err := request(http.MethodGet, os.Args[2], "", "")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(body)
	case "post":
		if len(os.Args) != 5 {
			os.Exit(2)
		}
		body, err := request(http.MethodPost, os.Args[2], os.Args[3], os.Args[4])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(body)
	case "wait-ready":
		if len(os.Args) != 5 {
			os.Exit(2)
		}
		attempts, _ := strconv.Atoi(os.Args[3])
		sleepMS, _ := strconv.Atoi(os.Args[4])
		for i := 0; i < attempts; i++ {
			body, err := request(http.MethodGet, os.Args[2], "", "")
			if err == nil && strings.Contains(body, `"status":"ready"`) {
				fmt.Print(body)
				return
			}
			time.Sleep(time.Duration(sleepMS) * time.Millisecond)
		}
		os.Exit(1)
	default:
		os.Exit(2)
	}
}
GO

port="${PORT:-18080}"
ADDR="127.0.0.1:${port}" SQLITE_PATH="${tmpdir}/smoke.db" GOPROXY=off go run ./cmd/server >"${tmpdir}/server.log" 2>&1 &
pid="$!"

if ! go run "$probe" wait-ready "http://127.0.0.1:${port}/readyz" 60 250 >/dev/null 2>&1; then
  echo "service did not become ready" >&2
  cat "${tmpdir}/server.log" >&2 || true
  exit 1
fi

response="$(go run "$probe" post "http://127.0.0.1:${port}/api/v1/sterilization-runs" '{"device_id":"smoke-autoclave","business_key":"smoke-run"}' smoke-create)"

if [[ "$response" != *'"revision_id":"rev-'* ]] || [[ "$response" != *'"state":"DRAFT"'* ]]; then
  echo "unexpected create response: $response" >&2
  exit 1
fi

health="$(go run "$probe" get "http://127.0.0.1:${port}/healthz")"
if [[ "$health" != *'"status":"ok"'* ]]; then
  echo "unexpected health response: $health" >&2
  exit 1
fi

echo "smoke ok"
