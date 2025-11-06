/**
 * 认证相关 API 调用
 */
const AuthAPI = {
    async sendCode(email, type = 0) {
        const formData = new FormData();
        formData.append('email', email);
        formData.append('type', type);
        return await request('/auth/send-code', 'POST', formData);
    },

    async login(email, password) {
        const formData = new FormData();
        formData.append('email', email);
        formData.append('password', password);
        return await request('/auth/login', 'POST', formData);
    },

    async register(userData) {
        const { email, nickname, password, verifiedKey, inputCode } = userData;
        const formData = new FormData();
        formData.append('email', email);
        formData.append('nickname', nickname);
        formData.append('password', password);
        formData.append('verifiedKey', verifiedKey);
        formData.append('inputCode', inputCode);
        return await request('/auth/register', 'POST', formData);
    }
};

/**
 * UI 辅助函数
 */
const UIHelper = {
    toggleButtonLoading(btnId, isLoading) {
        const btn = document.getElementById(btnId);
        const btnText = document.getElementById(`${btnId.replace('-btn', '-btn-text')}`);
        const btnSpinner = document.getElementById(`${btnId.replace('-btn', '-btn-spinner')}`);

        if (btn) btn.disabled = isLoading;
        if (btnText) btnText.classList.toggle('hidden', isLoading);
        if (btnSpinner) btnSpinner.classList.toggle('hidden', !isLoading);
    },

    startCountdown(btnId, textId, seconds = 60) {
        const btn = document.getElementById(btnId);
        const text = document.getElementById(textId);
        const originalText = text.textContent;

        btn.disabled = true;
        let countdown = seconds;

        const interval = setInterval(() => {
            countdown--;
            text.textContent = `Resend (${countdown}s)`;

            if (countdown <= 0) {
                clearInterval(interval);
                btn.disabled = false;
                text.textContent = originalText;
            }
        }, 1000);

        return interval;
    }
};

console.log('auth.js loaded');

// ==================== 事件监听绑定 ====================
document.addEventListener('DOMContentLoaded', function() {
    console.log('Binding event listeners...');

    // ==================== 登录表单 ====================
    const loginForm = document.getElementById('loginFormElement');
    if (loginForm) {
        loginForm.addEventListener('submit', async function(e) {
            e.preventDefault();

            const email = document.getElementById('login-email').value.trim();
            const password = document.getElementById('login-password').value;

            if (!email || !password) {
                Toast.warning('Please fill in all fields');
                return;
            }

            if (!Validators.isValidEmail(email)) {
                Toast.error('Please enter a valid email address', 'Invalid Email');
                return;
            }

            UIHelper.toggleButtonLoading('login-submit-btn', true);

            try {
                // 🔐 加密密码
                const encryptedPassword = CryptoUtils.md5(password);
                console.log('Original password length:', password.length);

                const result = await AuthAPI.login(email, encryptedPassword);  // ← 传入加密后的密码

                if (result.success) {
                    Toast.success('Welcome back, ' + (result.data.nickname || 'User') + '!', 'Login Successful');
                    localStorage.setItem('userInfo', JSON.stringify(result.data));

                    setTimeout(() => {
                         window.location.href = '/api/';
                    }, 1500);
                } else {
                    Toast.error(result.message || 'Please check your credentials', 'Login Failed');
                }
            } catch (error) {
                console.error('Login error:', error);
                Toast.error('Please try again later', 'Network Error');
            } finally {
                UIHelper.toggleButtonLoading('login-submit-btn', false);
            }
        });
    }

    // ==================== 发送验证码 ====================
    const sendCodeBtn = document.getElementById('send-code-btn');
    if (sendCodeBtn) {
        sendCodeBtn.addEventListener('click', async function() {
            const email = document.getElementById('reg-email').value.trim();

            if (!email) {
                Toast.warning('Please enter your email address first');
                return;
            }

            if (!Validators.isValidEmail(email)) {
                Toast.error('Please enter a valid email address', 'Invalid Email');
                return;
            }

            const sendCodeText = document.getElementById('send-code-text');
            const originalText = sendCodeText.textContent;

            sendCodeBtn.disabled = true;
            sendCodeText.textContent = 'Sending...';

            try {
                const result = await AuthAPI.sendCode(email, 0);

                if (result.success) {
                    document.getElementById('verifiedKey').value = result.data;

                    Toast.success(
                        `A 6-digit code has been sent to ${email}. Please check your inbox.`,
                        '✉️ Code Sent'
                    );

                    UIHelper.startCountdown('send-code-btn', 'send-code-text', 60);
                } else {
                    Toast.error(result.message || 'Please try again', 'Failed to Send Code');
                    sendCodeBtn.disabled = false;
                    sendCodeText.textContent = originalText;
                }
            } catch (error) {
                console.error('Send code error:', error);
                Toast.error('Please check your connection', 'Network Error');
                sendCodeBtn.disabled = false;
                sendCodeText.textContent = originalText;
            }
        });
    }

    // ==================== 注册表单 ====================
    const registerForm = document.getElementById('registerFormElement');
    if (registerForm) {
        registerForm.addEventListener('submit', async function(e) {
            e.preventDefault();

            const nickname = document.getElementById('reg-nickname').value.trim();
            const email = document.getElementById('reg-email').value.trim();
            const password = document.getElementById('reg-password').value;
            const code = document.getElementById('reg-code').value.trim();
            const verifiedKey = document.getElementById('verifiedKey').value;
            const termsChecked = document.getElementById('terms').checked;

            // 验证
            if (!nickname || !email || !password || !code) {
                Toast.warning('Please fill in all required fields');
                return;
            }

            if (!Validators.isValidNickname(nickname)) {
                Toast.warning('Nickname must be 2-20 characters');
                return;
            }

            if (!Validators.isValidEmail(email)) {
                Toast.error('Please enter a valid email address', 'Invalid Email');
                return;
            }

            if (!Validators.isValidPassword(password)) {
                Toast.warning('Password must be at least 8 characters');
                return;
            }

            if (!Validators.isValidCode(code)) {
                Toast.warning('Verification code must be 6 digits');
                return;
            }

            if (!verifiedKey) {
                Toast.warning('Please request a verification code first');
                return;
            }

            if (!termsChecked) {
                Toast.warning('You must agree to the Terms and Privacy Policy');
                return;
            }

            UIHelper.toggleButtonLoading('register-submit-btn', true);

            try {
                const result = await AuthAPI.register({
                    email,
                    nickname,
                    password,
                    verifiedKey,
                    inputCode: code
                });

                if (result.success) {
                    Toast.success(
                        'Your account has been created successfully!',
                        '🎉 Welcome'
                    );

                    setTimeout(() => {
                        showLogin(new Event('click'));
                        registerForm.reset();
                        document.getElementById('verifiedKey').value = '';
                    }, 2000);
                } else {
                    Toast.error(result.message || 'Please try again', 'Registration Failed');
                }
            } catch (error) {
                console.error('Register error:', error);
                Toast.error('Please try again later', 'Network Error');
            } finally {
                UIHelper.toggleButtonLoading('register-submit-btn', false);
            }
        });
    }

    console.log('Event listeners bound successfully!');
});