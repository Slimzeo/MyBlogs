package com.myblogs.entity.dto;

import lombok.Data;

@Data
public class JwtUserInfoDto {

    private Long userId;

    private String email;

    private String nickname;

    /**
     * 账号封禁状态
     */
    private Integer status;

    private Long issuedTime;
    private Long expiredTime;
}
