package com.myblogs.entity.enums;


public enum EmailCodeTypeEnum {
    REGISTER(0, "欢迎注册 MyBlogs"),
    RESET_PWD(1, "重新设置您的密码");

    private Integer type;
    private String description;
    EmailCodeTypeEnum(Integer type, String description) {
        this.type = type;
        this.description = description;
    }

    static public EmailCodeTypeEnum getByType(Integer type) {
        for (EmailCodeTypeEnum e : EmailCodeTypeEnum.values()) {
            if (e.type.equals(type)) {
                return e;
            }
        }
        return null;
    }


    public Integer getType() {
        return type;
    }

    public String getDescription() {
        return description;
    }
}
