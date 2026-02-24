package luascripts

const LikeScript = `
local users_key = KEYS[1]
local count_key = KEYS[2]
local user_id = ARGV[1]

local added = redis.call('SADD', users_key, user_id)
if tonumber(added) == 1 then
  local count = redis.call('INCR', count_key)
  return {1, tonumber(count)}
end

local count = redis.call('GET', count_key)
if not count then
  count = redis.call('SCARD', users_key)
  redis.call('SET', count_key, count)
end
return {0, tonumber(count)}
`

const UnlikeScript = `
local users_key = KEYS[1]
local count_key = KEYS[2]
local user_id = ARGV[1]

local removed = redis.call('SREM', users_key, user_id)
if tonumber(removed) == 1 then
  local count = redis.call('DECR', count_key)
  if tonumber(count) < 0 then
    redis.call('SET', count_key, 0)
    count = 0
  end
  return {1, tonumber(count)}
end

local final = redis.call('GET', count_key)
if not final then
  final = redis.call('SCARD', users_key)
  redis.call('SET', count_key, final)
end
return {0, tonumber(final)}
`

const ApplyCollectStateScript = `
local users_key = KEYS[1]
local count_key = KEYS[2]
local user_id = ARGV[1]
local desired = tonumber(ARGV[2])

local exists = redis.call('SISMEMBER', users_key, user_id)
if desired == 1 then
  if exists == 0 then
    redis.call('SADD', users_key, user_id)
    redis.call('INCR', count_key)
  end
else
  if exists == 1 then
    redis.call('SREM', users_key, user_id)
    local count = redis.call('DECR', count_key)
    if tonumber(count) < 0 then
      redis.call('SET', count_key, 0)
    end
  end
end

local final = redis.call('GET', count_key)
if not final then
  final = '0'
end
return tonumber(final)
`
