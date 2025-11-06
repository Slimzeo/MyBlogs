/**
 * 表单验证工具
 */
const Validators = {
    /**
     * 验证邮箱格式
     */
    isValidEmail(email) {
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        return emailRegex.test(email);
    },

    /**
     * 验证密码强度（至少8位）
     */
    isValidPassword(password) {
        return password && password.length >= 8;
    },

    /**
     * 验证验证码（6位数字）
     */
    isValidCode(code) {
        return code && code.length === 6 && /^\d{6}$/.test(code);
    },

    /**
     * 验证昵称（不为空，长度在2-20之间）
     */
    isValidNickname(nickname) {
        return nickname && nickname.trim().length >= 2 && nickname.trim().length <= 20;
    }
};

console.log('validators.js loaded');