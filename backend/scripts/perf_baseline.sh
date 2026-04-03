#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
U1="${U1:-u1776321964a}"
U2="${U2:-u1776321964b}"
PW="${PW:-Passw0rd!}"

REQ_FEED="${REQ_FEED:-300}"
REQ_DETAIL="${REQ_DETAIL:-500}"
REQ_LIKE="${REQ_LIKE:-1000}"
REQ_COLLECT="${REQ_COLLECT:-400}"
REQ_RANK="${REQ_RANK:-500}"
REQ_FOLLOW="${REQ_FOLLOW:-300}"

CONC_FEED="${CONC_FEED:-50}"
CONC_DETAIL="${CONC_DETAIL:-80}"
CONC_LIKE="${CONC_LIKE:-120}"
CONC_COLLECT="${CONC_COLLECT:-80}"
CONC_RANK="${CONC_RANK:-80}"
CONC_FOLLOW="${CONC_FOLLOW:-60}"

OUT_DIR="${OUT_DIR:-./result/perf}"
mkdir -p "$OUT_DIR"
REPORT="$OUT_DIR/baseline_$(date +%Y%m%d_%H%M%S).log"

log() {
  echo "$@" | tee -a "$REPORT"
}

extract_token() {
  sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'
}

extract_id() {
  sed -n 's/.*"id":"\{0,1\}\([0-9][0-9]*\)".*/\1/p' | head -n1
}

wait_health() {
  local i
  for i in $(seq 1 30); do
    if curl -sS "$BASE_URL/health" | grep -q '"status":"ok"'; then
      return 0
    fi
    sleep 1
  done
  return 1
}

ensure_user() {
  local user="$1"
  local body
  body=$(curl -sS -X POST "$BASE_URL/api/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$user\",\"password\":\"$PW\"}" || true)
  if ! echo "$body" | grep -q '"code"'; then
    log "WARN register response parse failed for $user: $body"
  fi
}

login() {
  local user="$1"
  local body token
  body=$(curl -sS -X POST "$BASE_URL/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$user\",\"password\":\"$PW\"}")
  token=$(echo "$body" | extract_token)
  if [[ -z "$token" ]]; then
    log "ERROR login failed for $user: $body"
    exit 1
  fi
  echo "$token"
}

ensure_article() {
  local t1="$1"
  local aid create_body

  aid=$(curl -sS "$BASE_URL/api/feed/following?limit=1" \
    -H "Authorization: Bearer $t1" | extract_id)
  if [[ -n "$aid" ]]; then
    echo "$aid"
    return 0
  fi

  create_body=$(curl -sS -X POST "$BASE_URL/api/articles" \
    -H "Authorization: Bearer $t1" \
    -H 'Content-Type: application/json' \
    -d '{"title":"perf seed article","summary":"seed","content_md":"seed content for perf"}')
  aid=$(echo "$create_body" | extract_id)
  if [[ -z "$aid" ]]; then
    log "ERROR cannot create seed article: $create_body"
    exit 1
  fi
  echo "$aid"
}

run_case() {
  local name="$1"
  local requests="$2"
  local conc="$3"
  local req_cmd="$4"

  local tmp
  tmp=$(mktemp)
  local start_ms end_ms dur_ms
  start_ms=$(date +%s%3N)
  set +e
  seq 1 "$requests" | xargs -I{} -P "$conc" sh -c "$req_cmd" > "$tmp"
  set -e
  end_ms=$(date +%s%3N)
  dur_ms=$((end_ms - start_ms))
  if [[ "$dur_ms" -le 0 ]]; then
    dur_ms=1
  fi

  local qps err err_rate n p50_idx p95_idx p99_idx p50 p95 p99
  qps=$(awk -v r="$requests" -v d="$dur_ms" 'BEGIN{printf "%.2f", r*1000/d}')
  err=$(awk '$1 !~ /^2/ {e++} END{print e+0}' "$tmp")
  err_rate=$(awk -v e="$err" -v r="$requests" 'BEGIN{printf "%.2f", (e*100.0)/r}')
  n=$(wc -l < "$tmp")

  p50_idx=$(awk -v n="$n" 'BEGIN{i=int(n*0.50); if(i<1)i=1; print i}')
  p95_idx=$(awk -v n="$n" 'BEGIN{i=int(n*0.95); if(i<1)i=1; print i}')
  p99_idx=$(awk -v n="$n" 'BEGIN{i=int(n*0.99); if(i<1)i=1; print i}')

  p50=$(awk '{print $2}' "$tmp" | sort -n | sed -n "${p50_idx}p")
  p95=$(awk '{print $2}' "$tmp" | sort -n | sed -n "${p95_idx}p")
  p99=$(awk '{print $2}' "$tmp" | sort -n | sed -n "${p99_idx}p")

  log "CASE=$name REQ=$requests CONC=$conc QPS=$qps ERR=${err_rate}% P50=${p50}s P95=${p95}s P99=${p99}s"
  rm -f "$tmp"
}

log "=== baseline start at $(date '+%F %T') ==="
log "BASE_URL=$BASE_URL"

if ! wait_health; then
  log "ERROR service health check failed at $BASE_URL/health"
  exit 1
fi

ensure_user "$U1"
ensure_user "$U2"

T1=$(login "$U1")
T2=$(login "$U2")

AID=$(ensure_article "$T1")

# Ensure relation exists for feed/follow scenarios.
curl -sS -o /dev/null -X POST -H "Authorization: Bearer $T1" "$BASE_URL/api/follow/2" || true
curl -sS -o /dev/null -X POST -H "Authorization: Bearer $T2" "$BASE_URL/api/follow/1" || true

log "seed article id=$AID"

run_case "feed_following" "$REQ_FEED" "$CONC_FEED" \
  "curl --max-time 5 -s -o /dev/null -w '%{http_code} %{time_total}\\n' -H 'Authorization: Bearer $T1' '$BASE_URL/api/feed/following?limit=20'"

run_case "article_detail" "$REQ_DETAIL" "$CONC_DETAIL" \
  "curl --max-time 5 -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/articles/$AID'"

run_case "like_burst" "$REQ_LIKE" "$CONC_LIKE" \
  "curl --max-time 5 -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST -H 'Authorization: Bearer $T1' '$BASE_URL/api/articles/$AID/like'"

run_case "collect_toggle" "$REQ_COLLECT" "$CONC_COLLECT" \
  "curl --max-time 5 -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST -H 'Authorization: Bearer $T1' '$BASE_URL/api/articles/$AID/collect'"

run_case "follow_toggle" "$REQ_FOLLOW" "$CONC_FOLLOW" \
  "curl --max-time 5 -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST -H 'Authorization: Bearer $T1' '$BASE_URL/api/follow/2'"

run_case "hot_rank" "$REQ_RANK" "$CONC_RANK" \
  "curl --max-time 5 -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/rank/hot?window=24h&limit=20'"

log "=== baseline done at $(date '+%F %T') ==="
log "REPORT=$REPORT"
