// api.js - API请求封装层

// API基础配置
const API_CONFIG = {
    baseURL: 'http://localhost:2233/api',
    timeout: 10000,
    headers: {
        'Content-Type': 'application/json'
    }
};

/**
 * 通用请求函数
 * @param {string} url - 请求URL
 * @param {string} method - 请求方法
 * @param {object} data - 请求数据
 * @returns {Promise} - 返回Promise对象
 */
async function request(url, method = 'GET', data = null) {
    const options = {
        method: method,
        headers: { ...API_CONFIG.headers }
    };

    // 如果是POST请求且有数据
    if (method === 'POST' && data) {
        // 根据数据类型决定如何发送
        if (data instanceof FormData) {
            // FormData不需要设置Content-Type，浏览器会自动设置
            delete options.headers['Content-Type'];
            options.body = data;
        } else if (typeof data === 'object') {
            // 对象转JSON
            options.body = JSON.stringify(data);
        } else {
            options.body = data;
        }
    }

    try {
        const response = await fetch(API_CONFIG.baseURL + url, options);
        const result = await response.json();

        // 根据你的后端ResponseVO结构，检查status或code
        // 假设成功时 status 为 'success' 或 code 为 200
        if (result.status === 'success' || result.code === 200) {
            return {
                success: true,
                data: result.data,
                message: result.info || result.message || 'Success'
            };
        } else {
            return {
                success: false,
                data: null,
                message: result.info || result.message || 'Request failed'
            };
        }
    } catch (error) {
        console.error('API Request Error:', error);
        return {
            success: false,
            data: null,
            message: error.message || 'Network error'
        };
    }
}