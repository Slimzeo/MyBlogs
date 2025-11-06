package com.myblogs.entity.enums;

public enum UserInfoStatusEnum {
    ACTIVE(0, "active"),
    FREE(1, "forbidden"),
    DELETE(2, "deleted");

    private Integer status;
    private String desc;
    UserInfoStatusEnum(Integer status, String desc) {
        this.status = status;
        this.desc = desc;
    }



    public Integer getStatus() {
        return status;
    }

    public String getDesc() {
        return desc;
    }
}
