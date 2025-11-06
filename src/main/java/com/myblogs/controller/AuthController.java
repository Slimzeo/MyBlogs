package com.myblogs.controller;

import com.myblogs.entity.vo.ResponseVO;
import com.myblogs.entity.vo.UserInfoVO;
import com.myblogs.exception.BusinessException;
import com.myblogs.redis.RedisComponent;
import com.myblogs.service.EmailService;
import com.myblogs.service.UserInfoService;
import jakarta.annotation.Resource;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotEmpty;
import jakarta.validation.constraints.NotNull;
import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController("authController")
@RequestMapping("/auth")
public class AuthController extends ABaseController{

    @Resource
    UserInfoService userInfoService;

    @Resource
    EmailService emailService;

    @Resource
    RedisComponent redisComponent;

    @PostMapping("/send-code")
    public ResponseVO sendCode(@NotEmpty @Email String email,
                               @NotNull Integer type) {
        String verifiedKey = this.emailService.sendEmailCode(email, type);
        // 真正的verified code被存在redis里面, 按照<verifiedKey, verifiedCode>
        // 所以只需要返回key
        return getSuccessResponseVO(verifiedKey);
    }


    @PostMapping("/login")
    public ResponseVO login(@NotEmpty @Email String email,
                            @NotEmpty String password) {
        UserInfoVO userInfoVO = this.userInfoService.login(email, password);

        // 我们需要给前端返回用户信息, 如session, cookie, token.... 以便之后的操作鉴权
        return getSuccessResponseVO(userInfoVO);
    }

    @PostMapping("/register")
    public ResponseVO register(@NotEmpty @Email String email,
                               @NotEmpty String nickname,
                               @NotEmpty String password,
                               @NotEmpty String verifiedKey,
                               @NotEmpty String inputCode) {
        try {
            String checkCode = redisComponent.getEmailCheckInfo(verifiedKey);
            if (! inputCode.equals(checkCode)) {
                throw new BusinessException("邮箱验证码不正确");
            }
            this.userInfoService.register(email, nickname, password);
        } finally {
            redisComponent.deleteEmailCheckInfo(verifiedKey);
        }

        return getSuccessResponseVO(null);
    }



}
