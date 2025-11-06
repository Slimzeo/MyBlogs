package com.myblogs.entity.constants;

public class Constants {
    public static final String REDIS_KEY_CHECK_CODE = "myblogs:mailcode";

    public static final String MAIL_TILE_SUBJECT = "[MyBlogs] 邮箱验证码";


    public static final Integer REDIS_EXPIRE_ONE_MINUTE = 60;
    public static final Integer REDIS_MAIL_CHECK_EXPIRE = 60*5;


    public static final String EMAIL_TEMPLATE = "email/verification-code";
    public static final String REDIS_KEY_VERIFICATION_CODE = "verification-code";


    public static final String DEFAULT_DESCRIPTION = "这个人很懒, 什么都没有写";

}
