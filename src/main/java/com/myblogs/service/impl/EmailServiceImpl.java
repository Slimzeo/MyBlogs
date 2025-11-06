package com.myblogs.service.impl;

import com.myblogs.entity.config.AppConfig;
import com.myblogs.entity.constants.Constants;
import com.myblogs.entity.enums.EmailCodeTypeEnum;
import com.myblogs.entity.enums.ResponseCodeEnum;
import com.myblogs.exception.BusinessException;
import com.myblogs.redis.RedisComponent;
import com.myblogs.service.EmailService;
import com.myblogs.utils.StringTools;
import jakarta.annotation.Resource;
import jakarta.mail.MessagingException;
import jakarta.mail.internet.MimeMessage;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.mail.javamail.JavaMailSender;
import org.springframework.mail.javamail.MimeMessageHelper;
import org.springframework.stereotype.Service;
import org.thymeleaf.TemplateEngine;
import org.thymeleaf.context.Context;


// TODO 完善email发送验证码, 并把<key,code>存进redis -> 配置redis相关服务
@Service("emailService")
public class EmailServiceImpl implements EmailService {
    private static final Logger logger = LoggerFactory.getLogger(EmailServiceImpl.class);

    @Resource
    private RedisComponent redisComponent;

    @Resource
    private AppConfig appConfig;

    @Resource
    private JavaMailSender mailSender;

    @Resource
    private TemplateEngine templateEngine;

    @Override
    public String sendEmailCode(String email, Integer type) {
        try {
            String code = StringTools.getVerificationCode();
            String key = StringTools.getVerificationKey(email, type);
            this.sendEmail(email, code, type);
            redisComponent.saveEmailCheckInfo(key, code);
            logger.debug("验证码已发送到email:{}",email);
            return key;
        } catch (Exception e) {
            logger.debug("send email code to email:{} fails, please check", email,e);
            return null;
        }
    }


    @Override
    public Boolean checkEmailCode(String key, String inputCode) {
        String rightCode = redisComponent.getEmailCheckInfo(key);
        if (rightCode == null) {
            logger.warn("验证码过期或不存在:{}", key);
            return false;
        }
        if (!rightCode.equals(inputCode)) {
            return false;
        }
        redisComponent.deleteEmailCheckInfo(key);
        return true;
    }



    private void sendEmail(String toEmail, String code, Integer type) throws MessagingException {
        MimeMessage message = mailSender.createMimeMessage();
        MimeMessageHelper helper = new MimeMessageHelper(message, true, "UTF-8");
        helper.setTo(toEmail);
        helper.setFrom(appConfig.getMailUsername());
        helper.setSubject(Constants.MAIL_TILE_SUBJECT);

        Context context = new Context();
        EmailCodeTypeEnum typeEnum = EmailCodeTypeEnum.getByType(type);
        if (typeEnum == null) {
            throw new BusinessException(ResponseCodeEnum.CODE_600);
        }
        context.setVariable("title", typeEnum.getDescription());
        context.setVariable("code", code);
        context.setVariable("expireMinutes", Constants.REDIS_MAIL_CHECK_EXPIRE / 60);

        String content = templateEngine.process(Constants.EMAIL_TEMPLATE, context);
        helper.setText(content, true);

        mailSender.send(message);
    }

}
