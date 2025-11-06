package com.myblogs.entity.config;


import lombok.Data;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

@Data
@Component("appConfig")
public class AppConfig {

    @Value("${spring.data.redis.port}")
    Integer redisPort;
    @Value("${spring.data.redis.host}")
    String redisHost;


    @Value("${spring.mail.username}")
    String mailUsername;
    @Value("${spring.mail.host}")
    String mailHost;
    @Value("${spring.mail.password}")
    String mailPassword;
    @Value("${spring.mail.port}")
    Integer mailPort;


    @Value("${jwt.secret}")
    String jwtSecret;
    @Value("${jwt.expiration}")
    Long jwtExpiration;

    @Value("${admin.email}")
    String adminEmail;
}
