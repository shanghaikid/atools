#!/usr/bin/env bash
# Test script for agix DeepSeek integration.
# Usage: ./test_deepseek.sh [proxy_url]
#
# Prerequisites:
#   1. Configure DeepSeek key in ~/.agix/config.yaml:
#        keys:
#          deepseek: "sk-your-key"
#   2. Start agix:  agix start

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"

GREEN='\033[0;32m'
RED='\033[0;31m'
DIM='\033[2m'
RESET='\033[0m'

pass() { echo -e "${GREEN}PASS${RESET} $1"; }
fail() { echo -e "${RED}FAIL${RESET} $1"; exit 1; }

echo "Testing agix DeepSeek support against ${BASE_URL}"
echo ""

# --- Test 1: Health check ---
echo -e "${DIM}1. Health check${RESET}"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/health")
[ "$HTTP_CODE" = "200" ] && pass "GET /health → 200" || fail "GET /health → ${HTTP_CODE}"

# --- Test 2: Models list includes deepseek ---
echo -e "${DIM}2. Models list includes DeepSeek${RESET}"
MODELS=$(curl -s "${BASE_URL}/v1/models")
echo "$MODELS" | grep -q "deepseek-chat" && pass "deepseek-chat in /v1/models" || fail "deepseek-chat missing from /v1/models"
echo "$MODELS" | grep -q "deepseek-reasoner" && pass "deepseek-reasoner in /v1/models" || fail "deepseek-reasoner missing from /v1/models"

# --- Test 3: Chat completion with deepseek-chat ---
echo -e "${DIM}3. Chat completion (deepseek-chat)${RESET}"
RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-Agent-Name: test-script" \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "Say hello in one word."}],
    "max_tokens": 16
  }')

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
  pass "POST /v1/chat/completions → 200"
  # Show token usage from response headers (re-request with -i)
  HEADERS=$(curl -s -I "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "X-Agent-Name: test-script" \
    -d '{
      "model": "deepseek-chat",
      "messages": [{"role": "user", "content": "Say hi"}],
      "max_tokens": 8
    }' 2>/dev/null || true)

  CONTENT=$(echo "$BODY" | python3 -c "
import sys, json
r = json.load(sys.stdin)
c = r.get('choices', [{}])[0].get('message', {}).get('content', '')
print(c[:80])
" 2>/dev/null || echo "(could not parse)")
  echo "  Response: ${CONTENT}"
else
  fail "POST /v1/chat/completions → ${HTTP_CODE}\n  ${BODY}"
fi

# --- Test 4: Verify cost tracking ---
echo -e "${DIM}4. Cost tracking headers${RESET}"
RESP_HEADERS=$(curl -s -D - -o /dev/null "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-Agent-Name: test-script" \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "1+1=?"}],
    "max_tokens": 8
  }')

echo "$RESP_HEADERS" | grep -qi "X-Cost-USD" && pass "X-Cost-USD header present" || fail "X-Cost-USD header missing"
echo "$RESP_HEADERS" | grep -qi "X-Input-Tokens" && pass "X-Input-Tokens header present" || fail "X-Input-Tokens header missing"
echo "$RESP_HEADERS" | grep -qi "X-Output-Tokens" && pass "X-Output-Tokens header present" || fail "X-Output-Tokens header missing"

COST=$(echo "$RESP_HEADERS" | grep -i "X-Cost-USD" | tr -d '\r' | awk '{print $2}')
echo "  Cost: \$${COST}"

# --- Test 5: Streaming ---
echo -e "${DIM}5. Streaming (deepseek-chat)${RESET}"
STREAM_RESP=$(curl -s -w "\n%{http_code}" "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "X-Agent-Name: test-script" \
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "Count 1 to 3."}],
    "max_tokens": 32,
    "stream": true
  }')

STREAM_CODE=$(echo "$STREAM_RESP" | tail -1)
STREAM_BODY=$(echo "$STREAM_RESP" | sed '$d')

if [ "$STREAM_CODE" = "200" ]; then
  echo "$STREAM_BODY" | grep -q "data:" && pass "SSE stream received" || fail "No SSE data in stream"
else
  fail "Streaming → ${STREAM_CODE}"
fi

echo ""
echo -e "${GREEN}All tests passed.${RESET}"
