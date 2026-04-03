#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
OUT_DIR="${OUT_DIR:-./result/perf}"
REQ="${REQ:-100}"
CONC="${CONC:-20}"
TIMEOUT="${TIMEOUT:-5}"
PW="${PW:-Passw0rd!}"

mkdir -p "$OUT_DIR"
REPORT="$OUT_DIR/all_api_$(date +%Y%m%d_%H%M%S).log"

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

register_user() {
  local user="$1"
  local nick="$2"
  curl -sS -X POST "$BASE_URL/api/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$user\",\"password\":\"$PW\",\"nickname\":\"$nick\"}" || true
}

login_user() {
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

create_article() {
  local token="$1"
  local body aid
  body=$(curl -sS -X POST "$BASE_URL/api/articles" \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -d '{"title":"perf article","summary":"perf","content_md":"perf body","status":1}')
  aid=$(echo "$body" | extract_id)
  if [[ -z "$aid" ]]; then
    log "ERROR create article failed: $body"
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

log "=== perf all apis start $(date '+%F %T') ==="
log "BASE_URL=$BASE_URL REQ=$REQ CONC=$CONC"

if ! wait_health; then
  log "ERROR service health check failed"
  exit 1
fi

SUFFIX=$(date +%s)
U1="perf_${SUFFIX}_u1"
U2="perf_${SUFFIX}_u2"

register_user "$U1" "PerfU1" >/dev/null
register_user "$U2" "PerfU2" >/dev/null

T1=$(login_user "$U1")
T2=$(login_user "$U2")
AID=$(create_article "$T1")

# prepare relation for feed/follow API
# use known user id from self profile-independent way: follow by article author id
# create a second article with user2 and infer uid from article detail
AID2=$(create_article "$T2")
U2ID=$(curl -sS "$BASE_URL/api/articles/$AID2" | sed -n 's/.*"user_id":"\{0,1\}\([0-9][0-9]*\)".*/\1/p' | head -n1)
if [[ -z "$U2ID" ]]; then
  log "WARN cannot infer U2ID, set follow target to 1"
  U2ID=1
fi
curl -sS -o /dev/null -X POST -H "Authorization: Bearer $T1" "$BASE_URL/api/follow/$U2ID" || true

run_case "health" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/health'"

run_case "auth_register" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST '$BASE_URL/api/auth/register' -H 'Content-Type: application/json' -d '{\"username\":\"perf_reg_{}_${SUFFIX}\",\"password\":\"$PW\",\"nickname\":\"Perf{}\"}'"

run_case "auth_login" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST '$BASE_URL/api/auth/login' -H 'Content-Type: application/json' -d '{\"username\":\"$U1\",\"password\":\"$PW\"}'"

run_case "article_create" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST '$BASE_URL/api/articles' -H 'Authorization: Bearer $T1' -H 'Content-Type: application/json' -d '{\"title\":\"perf c {}\",\"summary\":\"s\",\"content_md\":\"body\",\"status\":1}'"

run_case "article_detail" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/articles/$AID'"

run_case "article_update" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X PUT '$BASE_URL/api/articles/$AID' -H 'Authorization: Bearer $T1' -H 'Content-Type: application/json' -d '{\"title\":\"upd\",\"summary\":\"upd\",\"content_md\":\"upd {}\",\"status\":1}'"

run_case "article_list" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/articles?limit=20'"

run_case "article_list_by_author" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/users/$U2ID/articles?limit=20'"

run_case "like" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST '$BASE_URL/api/articles/$AID/like' -H 'Authorization: Bearer $T1'"

run_case "unlike" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X DELETE '$BASE_URL/api/articles/$AID/like' -H 'Authorization: Bearer $T1'"

run_case "collect" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST '$BASE_URL/api/articles/$AID/collect' -H 'Authorization: Bearer $T1'"

run_case "uncollect" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X DELETE '$BASE_URL/api/articles/$AID/collect' -H 'Authorization: Bearer $T1'"

run_case "follow" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X POST '$BASE_URL/api/follow/$U2ID' -H 'Authorization: Bearer $T1'"

run_case "unfollow" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X DELETE '$BASE_URL/api/follow/$U2ID' -H 'Authorization: Bearer $T1'"

run_case "feed_following" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/feed/following?limit=20' -H 'Authorization: Bearer $T1'"

run_case "rank_hot" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/rank/hot?window=24h&limit=20'"

run_case "search" "$REQ" "$CONC" \
  "curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' '$BASE_URL/api/search?q=perf&limit=20'"

run_case "article_delete" "$REQ" "$CONC" \
  "aid=\$(curl --max-time $TIMEOUT -sS -X POST '$BASE_URL/api/articles' -H 'Authorization: Bearer $T2' -H 'Content-Type: application/json' -d '{\"title\":\"perf d {}\",\"summary\":\"s\",\"content_md\":\"body\",\"status\":1}' | sed -n 's/.*\"id\":\"\\{0,1\\}\\([0-9][0-9]*\\)\".*/\\1/p' | head -n1); curl --max-time $TIMEOUT -s -o /dev/null -w '%{http_code} %{time_total}\\n' -X DELETE '$BASE_URL/api/articles/'\"\$aid\" -H 'Authorization: Bearer $T2'"

log "NOTE admin APIs (/api/auth/ban,/api/auth/unban) are excluded unless an admin token is provisioned."
log "=== perf all apis done $(date '+%F %T') ==="
log "REPORT=$REPORT"
