#!/usr/bin/env bash
# demo-setup.sh — seed a fresh demo DB for the README GIF
# Usage: source scripts/demo-setup.sh  (sets MNEMOS_DATA_DIR for the session)
# Or:    bash scripts/demo-setup.sh && export MNEMOS_DATA_DIR=/tmp/mnemos-demo

set -euo pipefail

DEMO_DIR="${TMPDIR:-/tmp}/mnemos-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"

export MNEMOS_DATA_DIR="$DEMO_DIR"

echo "Demo DB: $DEMO_DIR"

# Seed memories that make the demo look good
# Session 1 memories — stored "yesterday"
mnemos store "JWT uses RS256, 1h expiry — config in auth/config.go" \
  --type long_term --tags "auth,jwt" --project demo

mnemos store "Fixed race condition in session handler — use sync.RWMutex not sync.Mutex" \
  --type episodic --tags "concurrency,session" --project demo

mnemos store "Build command: go build -ldflags '-X main.version=\$(cat VERSION)' ./..." \
  --type skill --tags "build,go" --project demo

mnemos store "Postgres connection pool: max 25 open, 5 idle, 30m lifetime — see db/pool.go" \
  --type long_term --tags "database,postgres" --project demo

mnemos store "Rate limiter uses token bucket — 100 req/min per user, config in middleware/ratelimit.go" \
  --type long_term --tags "api,rate-limit" --project demo

echo ""
echo "✅ Demo DB seeded with 5 memories"
echo "   Run: MNEMOS_DATA_DIR=$DEMO_DIR vhs scripts/demo.tape"
