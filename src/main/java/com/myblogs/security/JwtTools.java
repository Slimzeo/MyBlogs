package com.myblogs.security;

import com.myblogs.entity.config.AppConfig;
import com.myblogs.entity.dto.JwtUserInfoDto;
import com.myblogs.utils.ObjectUtils;
import io.jsonwebtoken.Claims;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.SignatureAlgorithm;
import io.jsonwebtoken.security.Keys;
import jakarta.annotation.Resource;
import org.springframework.stereotype.Component;

import java.nio.charset.StandardCharsets;
import java.security.Key;
import java.util.Date;
import java.util.Map;

// JSON Web Token
// jwt的核心在于优化redis查询的压力, 可以直接拿到可信的payload信息.
// header.payload.signature, 其中header和payload谁都能破解出来, 但是signature是根据私钥和payload组合算法生成出来的, 这是难点.
// 非对称主要是利用数学原理, 弄出了 私钥->公钥->signature, 因此公钥可以验证, 但是公钥无法反推回私钥, 所以无法伪造jwt.
// 我嫌麻烦, 我们就先用对称形式的jwt
@Component("jwtTools")
public class JwtTools {

    @Resource
    private AppConfig appConfig;

    private Key getSignSecretKey() {
        byte[] keyBytes = appConfig.getJwtSecret().getBytes(StandardCharsets.UTF_8);
        return Keys.hmacShaKeyFor(keyBytes);
    }

    public String generateJwtToken(JwtUserInfoDto jwtUserInfoDto) {
        Map<String ,Object> claims =  ObjectUtils.objectToMap(jwtUserInfoDto);
        long currentTime = System.currentTimeMillis();
        if (jwtUserInfoDto.getIssuedTime() == null) {
            claims.put("issuedTime", currentTime);
        }
        if (jwtUserInfoDto.getExpiredTime() == null) {
            claims.put("expiredTime", currentTime + appConfig.getJwtExpiration());
        }

        return Jwts.builder()
                .setClaims(claims)
                .setSubject(jwtUserInfoDto.getEmail())
                .setIssuedAt(new Date(jwtUserInfoDto.getIssuedTime() != null ?
                        jwtUserInfoDto.getIssuedTime() : currentTime))
                .setExpiration(new Date(jwtUserInfoDto.getExpiredTime() != null ?
                        jwtUserInfoDto.getExpiredTime() : currentTime + appConfig.getJwtExpiration()))
                .signWith(getSignSecretKey(), SignatureAlgorithm.HS256)
                .compact();
    }

    // get the payload
    public Claims parseJwtToken(String token) {
        return Jwts.parserBuilder()
                .setSigningKey(getSignSecretKey())
                .build()
                .parseClaimsJws(token)
                .getBody();
    }

    public JwtUserInfoDto getJwtUserInfoDto(String token) {
        return ObjectUtils.mapToObject(parseJwtToken(token), JwtUserInfoDto.class);
    }

    public boolean isTokenExpired(String token) {
        try {
            Claims claims = parseJwtToken(token);
            return claims.getExpiration().before(new Date());
        } catch (Exception e) {
            return false;
        }
    }

}
