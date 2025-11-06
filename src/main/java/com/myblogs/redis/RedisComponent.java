package com.myblogs.redis;


import com.myblogs.entity.constants.Constants;
import jakarta.annotation.Resource;
import org.springframework.stereotype.Component;

@Component("redisComponent")
public class RedisComponent {
    @Resource
    private RedisUtils redisUtils;

    public void saveEmailCheckInfo(String verifiedKey, String verifiedCode) {
        redisUtils.setex(verifiedKey, verifiedCode, Constants.REDIS_MAIL_CHECK_EXPIRE);
    }
    public String getEmailCheckInfo(String verifiedKey) {
        return redisUtils.get(verifiedKey);
    }
    public void deleteEmailCheckInfo(String verifiedKey) {
        redisUtils.delete(verifiedKey);
    }


}
