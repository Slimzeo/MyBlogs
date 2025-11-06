package com.myblogs.utils;

import java.lang.reflect.Field;
import java.util.HashMap;
import java.util.Map;

public class ObjectUtils {
    static public Map<String, Object> objectToMap(Object object) {
        Map<String, Object> map = new HashMap<String, Object>();
        Class<?> clazz = object.getClass();

        for (Field field : clazz.getDeclaredFields()) {
            try {
                field.setAccessible(true);
                Object value = field.get(object);

                // 只添加非null值（可选，根据需求调整）
                if (value != null) {
                    map.put(field.getName(), value);
                }
            } catch (IllegalAccessException e) {
                // 记录日志或处理异常
                throw new RuntimeException("Failed to convert object to map", e);
            }
        }

        return map;

    }

    public static <T> T mapToObject(Map<String, Object> map, Class<T> clazz) {
        try {
            T instance = clazz.getDeclaredConstructor().newInstance();

            for (Field field : clazz.getDeclaredFields()) {
                field.setAccessible(true);
                Object value = map.get(field.getName());

                if (value != null) {
                    // 处理类型转换（Integer -> Long等）
                    if (field.getType() == Long.class && value instanceof Integer) {
                        value = ((Integer) value).longValue();
                    } else if (field.getType() == Integer.class && value instanceof Long) {
                        value = ((Long) value).intValue();
                    }

                    field.set(instance, value);
                }
            }

            return instance;
        } catch (Exception e) {
            throw new RuntimeException("Failed to convert map to object", e);
        }
    }



}
