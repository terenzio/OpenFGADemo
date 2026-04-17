#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${SERVER_ADDR:-http://localhost:8000}"

echo "=== OpenFGA Demo: HTTP API Walkthrough ==="
echo ""

echo "--- Seeding demo data ---"
go run ./cmd/server -seed -exit
echo ""

echo "--- Starting server in background ---"
go run ./cmd/server &
SERVER_PID=$!
sleep 2

cleanup() {
    echo ""
    echo "--- Stopping server ---"
    kill $SERVER_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "--- As alice: list my documents ---"
curl -s -H "X-User-Id: alice" "$BASE_URL/documents" | jq .
echo ""

echo "--- As bob: try to view roadmap (should be 403) ---"
curl -s -w "\nHTTP %{http_code}\n" -H "X-User-Id: bob" "$BASE_URL/documents/roadmap"
echo ""

echo "--- As charlie: view roadmap (editor on folder:product) ---"
curl -s -H "X-User-Id: charlie" "$BASE_URL/documents/roadmap" | jq .
echo ""

echo "--- As diana: list documents (viewer on folder:company) ---"
curl -s -H "X-User-Id: diana" "$BASE_URL/documents" | jq .
echo ""

echo "--- As eve: list documents (acme member) ---"
curl -s -H "X-User-Id: eve" "$BASE_URL/documents" | jq .
echo ""

echo "--- As randomstranger: view public-memo (wildcard) ---"
curl -s -H "X-User-Id: randomstranger" "$BASE_URL/documents/public-memo" | jq .
echo ""

echo "--- As alice: create a new folder ---"
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"id":"engineering","name":"Engineering","parent_id":"company"}' \
  "$BASE_URL/folders" | jq .
echo ""

echo "--- As alice: create a new document ---"
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"id":"design-doc","title":"System Design","content":"Architecture overview","folder_id":"engineering"}' \
  "$BASE_URL/documents" | jq .
echo ""

echo "--- As alice: share design-doc with bob as editor ---"
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"user":"user:bob","relation":"editor"}' \
  "$BASE_URL/documents/design-doc/share" | jq .
echo ""

echo "--- As bob: edit the shared document ---"
curl -s -X PUT -H "X-User-Id: bob" -H "Content-Type: application/json" \
  -d '{"title":"System Design v2","content":"Updated architecture"}' \
  "$BASE_URL/documents/design-doc" | jq .
echo ""

echo "=== Demo complete! ==="
