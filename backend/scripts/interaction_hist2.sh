#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
PW="${PW:-Passw0rd!}"
REQ="${REQ:-200}"
CONC="${CONC:-20}"

SUF=$(date +%s)
U1="hist2_${SUF}_u1"
U2="hist2_${SUF}_u2"

register() {
  local user="$1"
  curl -sS -X POST "$BASE_URL/api/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${user}\",\"password\":\"${PW}\",\"nickname\":\"${user}\"}" >/dev/null
}

login() {
  local user="$1"
  curl -sS -X POST "$BASE_URL/api/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${user}\",\"password\":\"${PW}\"}" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'
}

create_article() {
  local token="$1"
  local title="$2"
  curl -sS -X POST "$BASE_URL/api/articles" \
    -H "Authorization: Bearer ${token}" \
    -H "Content-Type: application/json" \
    -d "{\"title\":\"${title}\",\"summary\":\"s\",\"content_md\":\"b\",\"status\":1}" | sed -n 's/.*"id":"\{0,1\}\([0-9][0-9]*\)".*/\1/p' | head -n1
}

register "$U1"
register "$U2"
T1=$(login "$U1")
T2=$(login "$U2")
AID=$(create_article "$T1" "hist2-a1")
AID2=$(create_article "$T2" "hist2-a2")
U2ID=$(curl -sS "$BASE_URL/api/articles/$AID2" | sed -n 's/.*"user_id":"\{0,1\}\([0-9][0-9]*\)".*/\1/p' | head -n1)

echo "REQ=$REQ CONC=$CONC AID=$AID U2ID=$U2ID"

seq 1 "$REQ" | xargs -I{} -P "$CONC" sh -c "curl --max-time 8 -s -o /dev/null -w '%{http_code}\n' -X POST '$BASE_URL/api/articles/$AID/collect' -H 'Authorization: Bearer $T1'" | sort | uniq -c | sed 's/^/collect /'
seq 1 "$REQ" | xargs -I{} -P "$CONC" sh -c "curl --max-time 8 -s -o /dev/null -w '%{http_code}\n' -X DELETE '$BASE_URL/api/articles/$AID/collect' -H 'Authorization: Bearer $T1'" | sort | uniq -c | sed 's/^/uncollect /'
seq 1 "$REQ" | xargs -I{} -P "$CONC" sh -c "curl --max-time 8 -s -o /dev/null -w '%{http_code}\n' -X POST '$BASE_URL/api/follow/$U2ID' -H 'Authorization: Bearer $T1'" | sort | uniq -c | sed 's/^/follow /'
seq 1 "$REQ" | xargs -I{} -P "$CONC" sh -c "curl --max-time 8 -s -o /dev/null -w '%{http_code}\n' -X DELETE '$BASE_URL/api/follow/$U2ID' -H 'Authorization: Bearer $T1'" | sort | uniq -c | sed 's/^/unfollow /'
