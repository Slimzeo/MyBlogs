package com.myblogs.redis;


import jakarta.annotation.Resource;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.util.Arrays;
import java.util.concurrent.TimeUnit;

@Component("redisUtils")
public class RedisUtils {
    @Resource
    private RedisTemplate<String, Object> redisTemplate;
    @Resource
    private StringRedisTemplate stringRedisTemplate;

    private static final Logger logger = LoggerFactory.getLogger(RedisUtils.class);

    public <V> V get(String key) {
        return key==null ? null:(V) redisTemplate.opsForValue().get(key);
    }

    public<V> boolean set(String key, V value) {
        try {
            redisTemplate.opsForValue().set(key, value);
            return true;
        } catch (Exception e) {
            logger.error("RedisUtils error when set redisKey:{}, value:{}", key, value, e);
            return false;
        }
    }

    public<V> boolean setex(String key, V value, long time) {
        try {
            if (time > 0) {
                redisTemplate.opsForValue().set(key, value, time, TimeUnit.SECONDS);
            } else {
                this.set(key, value);
            }
            return true;
        } catch (Exception e) {
            logger.error("RedisUtils error when setExpireAndValue redisKey:{}, value:{}", key, value, e);
            return false;
        }
    }

    public boolean setExpire(String key, long time) {
        try {
            if (time > 0) {redisTemplate.expire(key, time, TimeUnit.SECONDS);}
            return true;
        } catch (Exception e) {
            logger.error("RedisUtils error when setExpireTime redisKey:{}", key, e);
            return false;
        }
    }

    public void delete(String... key) {
        if (key != null && key.length > 0) {
            if (key.length == 1) { redisTemplate.delete(key[0]); }
            else redisTemplate.delete(Arrays.asList(key));
        }
    }
}
