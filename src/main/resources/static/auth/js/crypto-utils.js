/**
 * 加密工具函数
 */
const CryptoUtils = {
    /**
     * MD5 加密（兼容 Spring 的 DigestUtils.md5DigestAsHex）
     * @param {string} str - 要加密的字符串
     * @returns {string} - 32位小写 MD5 值
     */
    md5(str) {
        if (!str) return null;

        // 使用 crypto-js 的 MD5
        const hash = CryptoJS.MD5(str);

        // 转换为小写的十六进制字符串（与 Spring 的 DigestUtils.md5DigestAsHex 一致）
        return hash.toString(CryptoJS.enc.Hex);
    },

    /**
     * 验证 MD5 值是否匹配
     */
    verifyMd5(str, hash) {
        return this.md5(str) === hash;
    }
};

console.log('crypto-utils.js loaded');