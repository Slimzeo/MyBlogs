package com.myblogs.service;


import org.springframework.stereotype.Service;


public interface EmailService {

    String sendEmailCode(String email, Integer type);

    Boolean checkEmailCode(String key, String inputCode);


}
