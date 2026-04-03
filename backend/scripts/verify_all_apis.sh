#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
PW="${PW:-Passw0rd!}"
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

POSTGRES_USER="${POSTGRES_USER:-cgoforum}"
POSTGRES_DB="${POSTGRES_DB:-cgoforum}"
MEILI_ADDR="${MEILI_ADDR:-http://127.0.0.1:7700}"
MEILI_KEY="${MEILI_KEY:-cgoforum_master_key}"
MEILI_INDEX="${MEILI_INDEX:-articles}"

AUTO_RESTART_BACKEND_FOR_RANK="${AUTO_RESTART_BACKEND_FOR_RANK:-1}"

PASS_COUNT=0
FAIL_COUNT=0

log_info() { echo "[INFO] $*" >&2; }
log_pass() { PASS_COUNT=$((PASS_COUNT + 1)); echo "[PASS] $*" >&2; }
log_fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); echo "[FAIL] $*" >&2; }

extract_http_code() {
  sed -n 's/^__HTTP_CODE__:\([0-9][0-9][0-9]\)$/\1/p'
}

extract_http_body() {
  sed '/^__HTTP_CODE__:[0-9][0-9][0-9]$/d'
}

http_call() {
  curl -sS --retry 3 --retry-delay 1 --retry-connrefused --retry-all-errors "$@" -w '\n__HTTP_CODE__:%{http_code}\n'
}

json_contains_id() {
  local body="$1"
  local id="$2"
  if echo "$body" | grep -Eq '"id":"?'"$id"'"?' ; then
    return 0
  fi
  if echo "$body" | grep -Eq '"article_id":"?'"$id"'"?' ; then
    return 0
  fi
  return 1
}

extract_access_token() {
  sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p'
}

extract_id() {
  sed -n 's/.*"id":"\{0,1\}\([0-9][0-9]*\)".*/\1/p' | head -n1
}

extract_user_id() {
  sed -n 's/.*"user_id":"\{0,1\}\([0-9][0-9]*\)".*/\1/p' | head -n1
}

wait_health() {
  local i
  for i in $(seq 1 60); do
    local out code body
    out="$(http_call "$BASE_URL/health" || true)"
    code="$(echo "$out" | extract_http_code)"
    body="$(echo "$out" | extract_http_body)"
    if [[ "$code" == "200" ]] && echo "$body" | grep -q '"status":"ok"'; then
      return 0
    fi
    sleep 1
  done
  return 1
}

docker_compose_exec() {
  (cd "$ROOT_DIR" && docker compose "$@")
}

maybe_restart_backend() {
  if [[ "$AUTO_RESTART_BACKEND_FOR_RANK" != "1" ]]; then
    log_info "AUTO_RESTART_BACKEND_FOR_RANK=0，跳过 backend 重启"
    return 0
  fi

  if ! command -v docker >/dev/null 2>&1; then
    log_info "docker 不可用，跳过 backend 重启"
    return 0
  fi

  if ! docker_compose_exec ps >/dev/null 2>&1; then
    log_info "docker compose 不可用，跳过 backend 重启"
    return 0
  fi

  log_info "为触发热榜重建，重启 backend ..."
  if docker_compose_exec restart backend >/dev/null 2>&1; then
    if wait_health; then
      log_pass "backend 重启后健康检查通过"
      return 0
    fi
    log_fail "backend 重启后健康检查失败"
    return 1
  fi

  log_fail "backend 重启失败"
  return 1
}

require_tooling() {
  local missing=0
  for cmd in curl sed grep; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      log_fail "缺少命令: $cmd"
      missing=1
    fi
  done
  if [[ "$missing" -eq 1 ]]; then
    exit 1
  fi
}

assert_code_200() {
  local code="$1"
  local name="$2"
  local body="$3"
  if [[ "$code" == "200" ]]; then
    log_pass "$name -> HTTP 200"
  else
    log_fail "$name -> HTTP $code, body=$body"
  fi
}

poll_until() {
  local seconds="$1"
  local fn_name="$2"
  local i
  for i in $(seq 1 "$seconds"); do
    if "$fn_name"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

register_user() {
  local username="$1"
  local nickname="$2"
  local out code body
  out="$(http_call -X POST "$BASE_URL/api/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$username\",\"password\":\"$PW\",\"nickname\":\"$nickname\"}")"
  code="$(echo "$out" | extract_http_code)"
  body="$(echo "$out" | extract_http_body)"
  assert_code_200 "$code" "注册用户 $username" "$body"
}

login_user() {
  local username="$1"
  local out code body token
  out="$(http_call -X POST "$BASE_URL/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$username\",\"password\":\"$PW\"}")"
  code="$(echo "$out" | extract_http_code)"
  body="$(echo "$out" | extract_http_body)"
  assert_code_200 "$code" "登录用户 $username" "$body"

  token="$(echo "$body" | extract_access_token | head -n1)"
  if [[ -z "$token" ]]; then
    log_fail "登录后 access_token 为空: $username"
    return 1
  fi
  echo "$token"
}

create_article() {
  local token="$1"
  local title="$2"
  local summary="$3"
  local content_md="$4"

  local out code body aid
  out="$(http_call -X POST "$BASE_URL/api/articles" \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -d "{\"title\":\"$title\",\"summary\":\"$summary\",\"content_md\":\"$content_md\",\"status\":1}")"
  code="$(echo "$out" | extract_http_code)"
  body="$(echo "$out" | extract_http_body)"
  assert_code_200 "$code" "创建文章" "$body"

  aid="$(echo "$body" | extract_id)"
  if [[ -z "$aid" ]]; then
    log_fail "创建文章未返回 id"
    return 1
  fi
  echo "$aid"
}

main() {
  require_tooling

  log_info "开始全链路接口与数据面校验"
  log_info "BASE_URL=$BASE_URL"

  if wait_health; then
    log_pass "健康检查通过"
  else
    log_fail "健康检查失败: $BASE_URL/health"
    exit 1
  fi

  local suffix u1 u2 u3
  suffix="$(date +%s)"
  u1="verify_${suffix}_u1"
  u2="verify_${suffix}_u2"
  u3="verify_${suffix}_u3"

  register_user "$u1" "Verifier1"
  register_user "$u2" "Verifier2"
  register_user "$u3" "Verifier3"

  local t1 t2
  t1="$(login_user "$u1")"
  t2="$(login_user "$u2")"

  local cookie_file
  cookie_file="/tmp/cgoforum_verify_cookie_${suffix}.txt"

  local auth_out auth_code auth_body refresh_out refresh_code refresh_body logout_out logout_code logout_body refresh2_out refresh2_code
  auth_out="$(http_call -b "$cookie_file" -c "$cookie_file" -X POST "$BASE_URL/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$u3\",\"password\":\"$PW\"}")"
  auth_code="$(echo "$auth_out" | extract_http_code)"
  auth_body="$(echo "$auth_out" | extract_http_body)"
  assert_code_200 "$auth_code" "登录+写入refresh cookie" "$auth_body"

  refresh_out="$(http_call -b "$cookie_file" -c "$cookie_file" -X POST "$BASE_URL/api/auth/refresh")"
  refresh_code="$(echo "$refresh_out" | extract_http_code)"
  refresh_body="$(echo "$refresh_out" | extract_http_body)"
  assert_code_200 "$refresh_code" "刷新 access_token" "$refresh_body"

  local t3
  t3="$(echo "$auth_body" | extract_access_token | head -n1)"
  logout_out="$(http_call -b "$cookie_file" -c "$cookie_file" -X POST "$BASE_URL/api/auth/logout" \
    -H "Authorization: Bearer $t3")"
  logout_code="$(echo "$logout_out" | extract_http_code)"
  logout_body="$(echo "$logout_out" | extract_http_body)"
  assert_code_200 "$logout_code" "退出登录" "$logout_body"

  refresh2_out="$(http_call -b "$cookie_file" -c "$cookie_file" -X POST "$BASE_URL/api/auth/refresh")"
  refresh2_code="$(echo "$refresh2_out" | extract_http_code)"
  if [[ "$refresh2_code" == "401" ]]; then
    log_pass "退出后 refresh 返回 401"
  else
    log_fail "退出后 refresh 非 401，实际=$refresh2_code"
  fi

  local keyword title summary content aid
  keyword="vx_${suffix}_semantic_keyword"
  title="验证文档 $keyword"
  summary="summary $keyword"
  content="content $keyword"
  aid="$(create_article "$t2" "$title" "$summary" "$content")"

  local detail_out detail_code detail_body u2id
  detail_out="$(http_call "$BASE_URL/api/articles/$aid")"
  detail_code="$(echo "$detail_out" | extract_http_code)"
  detail_body="$(echo "$detail_out" | extract_http_body)"
  assert_code_200 "$detail_code" "文章详情" "$detail_body"
  u2id="$(echo "$detail_body" | extract_user_id)"
  if [[ -z "$u2id" ]]; then
    log_fail "无法从文章详情提取作者 user_id"
    u2id=0
  else
    log_pass "文章详情包含作者 user_id=$u2id"
  fi

  local upd_out upd_code upd_body
  upd_out="$(http_call -X PUT "$BASE_URL/api/articles/$aid" \
    -H "Authorization: Bearer $t2" \
    -H 'Content-Type: application/json' \
    -d "{\"title\":\"$title updated\",\"summary\":\"$summary updated\",\"content_md\":\"$content updated\",\"status\":1}")"
  upd_code="$(echo "$upd_out" | extract_http_code)"
  upd_body="$(echo "$upd_out" | extract_http_body)"
  assert_code_200 "$upd_code" "更新文章" "$upd_body"

  local list_out list_code list_body
  list_out="$(http_call "$BASE_URL/api/articles?limit=20")"
  list_code="$(echo "$list_out" | extract_http_code)"
  list_body="$(echo "$list_out" | extract_http_body)"
  assert_code_200 "$list_code" "文章列表" "$list_body"
  if json_contains_id "$list_body" "$aid"; then
    log_pass "文章列表包含新文章"
  else
    log_fail "文章列表未命中新文章 id=$aid"
  fi

  if [[ "$u2id" != "0" ]]; then
    local by_author_out by_author_code by_author_body
    by_author_out="$(http_call "$BASE_URL/api/users/$u2id/articles?limit=20")"
    by_author_code="$(echo "$by_author_out" | extract_http_code)"
    by_author_body="$(echo "$by_author_out" | extract_http_body)"
    assert_code_200 "$by_author_code" "作者文章列表" "$by_author_body"
  fi

  local like_out like_code collect_out collect_code unlike_out unlike_code uncollect_out uncollect_code
  like_out="$(http_call -X POST "$BASE_URL/api/articles/$aid/like" -H "Authorization: Bearer $t1")"
  like_code="$(echo "$like_out" | extract_http_code)"
  assert_code_200 "$like_code" "点赞" "$(echo "$like_out" | extract_http_body)"

  collect_out="$(http_call -X POST "$BASE_URL/api/articles/$aid/collect" -H "Authorization: Bearer $t1")"
  collect_code="$(echo "$collect_out" | extract_http_code)"
  assert_code_200 "$collect_code" "收藏" "$(echo "$collect_out" | extract_http_body)"

  if [[ "$u2id" != "0" ]]; then
    local follow_out follow_code feed_out feed_code unfollow_out unfollow_code
    follow_out="$(http_call -X POST "$BASE_URL/api/follow/$u2id" -H "Authorization: Bearer $t1")"
    follow_code="$(echo "$follow_out" | extract_http_code)"
    assert_code_200 "$follow_code" "关注作者" "$(echo "$follow_out" | extract_http_body)"

    feed_out="$(http_call "$BASE_URL/api/feed/following?limit=20" -H "Authorization: Bearer $t1")"
    feed_code="$(echo "$feed_out" | extract_http_code)"
    assert_code_200 "$feed_code" "关注流" "$(echo "$feed_out" | extract_http_body)"

    unfollow_out="$(http_call -X DELETE "$BASE_URL/api/follow/$u2id" -H "Authorization: Bearer $t1")"
    unfollow_code="$(echo "$unfollow_out" | extract_http_code)"
    assert_code_200 "$unfollow_code" "取消关注" "$(echo "$unfollow_out" | extract_http_body)"
  fi

  unlike_out="$(http_call -X DELETE "$BASE_URL/api/articles/$aid/like" -H "Authorization: Bearer $t1")"
  unlike_code="$(echo "$unlike_out" | extract_http_code)"
  assert_code_200 "$unlike_code" "取消点赞" "$(echo "$unlike_out" | extract_http_body)"

  uncollect_out="$(http_call -X DELETE "$BASE_URL/api/articles/$aid/collect" -H "Authorization: Bearer $t1")"
  uncollect_code="$(echo "$uncollect_out" | extract_http_code)"
  assert_code_200 "$uncollect_code" "取消收藏" "$(echo "$uncollect_out" | extract_http_body)"

  log_info "验证 Meilisearch 文档是否已上传 ..."
  meili_doc_ok() {
    local out code body
    out="$(http_call -H "Authorization: Bearer $MEILI_KEY" "$MEILI_ADDR/indexes/$MEILI_INDEX/documents/$aid" || true)"
    code="$(echo "$out" | extract_http_code)"
    body="$(echo "$out" | extract_http_body)"
    if [[ "$code" == "200" ]] && json_contains_id "$body" "$aid"; then
      return 0
    fi
    return 1
  }
  if poll_until 40 meili_doc_ok; then
    log_pass "文档上传到 Meilisearch 成功 (article_id=$aid)"
  else
    log_fail "文档未在 40s 内出现在 Meilisearch (article_id=$aid)"
  fi

  log_info "验证向量嵌入是否写入 pgvector 表 ..."
  embedding_ok() {
    local row
    row="$(docker_compose_exec exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tAc "select article_id from sys_article_embedding where article_id=$aid limit 1;" 2>/dev/null | tr -d '[:space:]' || true)"
    [[ "$row" == "$aid" ]]
  }
  if poll_until 40 embedding_ok; then
    log_pass "向量嵌入落库成功 (sys_article_embedding.article_id=$aid)"
  else
    log_fail "向量嵌入未在 40s 内落库 (article_id=$aid)"
  fi

  local search_out search_code search_body
  search_out="$(http_call "$BASE_URL/api/search?q=$keyword&limit=20")"
  search_code="$(echo "$search_out" | extract_http_code)"
  search_body="$(echo "$search_out" | extract_http_body)"
  assert_code_200 "$search_code" "语义搜索接口" "$search_body"
  if json_contains_id "$search_body" "$aid"; then
    log_pass "语义搜索命中新文章（向量检索链路可用）"
  else
    log_fail "语义搜索未命中新文章 id=$aid"
  fi

  local meili_search_out meili_search_code meili_search_body
  meili_search_out="$(http_call -X POST "$MEILI_ADDR/indexes/$MEILI_INDEX/search" \
    -H "Authorization: Bearer $MEILI_KEY" \
    -H 'Content-Type: application/json' \
    -d "{\"q\":\"$keyword\",\"limit\":5}")"
  meili_search_code="$(echo "$meili_search_out" | extract_http_code)"
  meili_search_body="$(echo "$meili_search_out" | extract_http_body)"
  if [[ "$meili_search_code" == "200" ]] && json_contains_id "$meili_search_body" "$aid"; then
    log_pass "Meilisearch 检索命中新文章（搜索引擎链路可用）"
  else
    log_fail "Meilisearch 检索未命中新文章或请求失败 (code=$meili_search_code)"
  fi

  local rank_before_out rank_before_body rank_after_out rank_after_body
  rank_before_out="$(http_call "$BASE_URL/api/rank/hot?window=24h&limit=50")"
  rank_before_body="$(echo "$rank_before_out" | extract_http_body)"

  # 提高文章热度，触发活动事件
  http_call -X POST "$BASE_URL/api/articles/$aid/like" -H "Authorization: Bearer $t1" >/dev/null || true
  http_call -X POST "$BASE_URL/api/articles/$aid/collect" -H "Authorization: Bearer $t1" >/dev/null || true
  http_call "$BASE_URL/api/articles/$aid" >/dev/null || true
  sleep 2

  rank_after_out="$(http_call "$BASE_URL/api/rank/hot?window=24h&limit=50")"
  rank_after_body="$(echo "$rank_after_out" | extract_http_body)"

  if json_contains_id "$rank_after_body" "$aid"; then
    log_pass "热榜接口已包含目标文章"
  else
    log_info "热榜未即时命中，尝试重启 backend 触发重建后复查"
    maybe_restart_backend || true
    rank_after_out="$(http_call "$BASE_URL/api/rank/hot?window=24h&limit=50")"
    rank_after_body="$(echo "$rank_after_out" | extract_http_body)"
    if json_contains_id "$rank_after_body" "$aid"; then
      log_pass "backend 重启后热榜已更新并包含目标文章"
    else
      log_fail "热榜未包含目标文章 (article_id=$aid)"
    fi
  fi

  local del_out del_code del_body
  del_out="$(http_call -X DELETE "$BASE_URL/api/articles/$aid" -H "Authorization: Bearer $t2")"
  del_code="$(echo "$del_out" | extract_http_code)"
  del_body="$(echo "$del_out" | extract_http_body)"
  assert_code_200 "$del_code" "删除文章" "$del_body"

  log_info "结果汇总: PASS=$PASS_COUNT FAIL=$FAIL_COUNT"
  if [[ "$FAIL_COUNT" -eq 0 ]]; then
    echo "ALL_CHECKS_PASSED"
    exit 0
  fi
  echo "CHECKS_FAILED"
  exit 1
}

main "$@"
