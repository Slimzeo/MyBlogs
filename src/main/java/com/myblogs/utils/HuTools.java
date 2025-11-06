package com.myblogs.utils;

import cn.hutool.core.lang.Snowflake;
import cn.hutool.core.util.IdUtil;

public class HuTools {
    private static final long DATACENTER_ID = 0L;

    // 机器 ID (0-31)，单机设置为 0 即可
    private static final long WORKER_ID = 0L;

    private static final Snowflake snowflake = IdUtil.getSnowflake(WORKER_ID, DATACENTER_ID);

    /**
     * 生成唯一的 Long 类型 ID
     */
    public static Long getNewUserId() {
        return snowflake.nextId();
    }

    /**
     * 生成唯一的 String 类型 ID
     */
    public static String nextIdStr() {
        return snowflake.nextIdStr();
    }
}
