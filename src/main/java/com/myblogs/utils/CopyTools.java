package com.myblogs.utils;

import org.springframework.beans.BeanUtils;

import java.util.ArrayList;
import java.util.List;

public class CopyTools {
    // 拷贝列表
    public static <T, S> List<T> copyList(List<S> sourceList, Class<T> targetClass) {
        List<T> targetList = new ArrayList<>();
        for (S source : sourceList) {
            T target = copy(source, targetClass);
            targetList.add(target);
        }
        return targetList;
    }

    // 拷贝单个对象
    public static <T, S> T copy(S source, Class<T> targetClass) {
        if (source == null) return null;
        try {
            T target = targetClass.getDeclaredConstructor().newInstance();
            BeanUtils.copyProperties(source, target);
            return target;
        } catch (Exception e) {
            throw new RuntimeException("对象拷贝失败", e);
        }
    }
}
