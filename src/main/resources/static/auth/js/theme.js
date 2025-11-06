/**
 * 主题管理器
 * 支持：自动跟随系统、手动切换、本地存储偏好
 */

class ThemeManager {
    constructor() {
        this.theme = null; // 'light' | 'dark' | null (跟随系统)
        this.images = {
            dark: '/api/assets/img/xiaohei_1.jpg',
            light: '/api/assets/img/xiaohei_2.jpg'
        };
        this.preloadImages(); // 预加载所有图片
        this.init();
    }

    preloadImages() {
        // 预加载两张图片到浏览器缓存
        Object.values(this.images).forEach(src => {
            const img = new Image();
            img.src = src;
        });
    }

    init() {
        // 从内存中读取用户偏好
        const savedTheme = this.getSavedTheme();

        if (savedTheme) {
            this.theme = savedTheme;
            this.applyTheme(savedTheme);
        } else {
            // 跟随系统
            this.followSystem();
        }

        // 监听系统主题变化
        this.watchSystemTheme();

        // 创建切换按钮
        this.createToggleButton();
    }

    getSavedTheme() {
        // 从内存变量中获取（页面刷新后会丢失，这是有意的设计）
        return window.__userThemePreference || null;
    }

    saveTheme(theme) {
        // 保存到内存变量
        window.__userThemePreference = theme;
    }

    followSystem() {
        const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        this.applyTheme(systemTheme);
    }

    watchSystemTheme() {
        const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
        mediaQuery.addEventListener('change', (e) => {
            // 只有在用户没有手动设置主题时才跟随系统
            if (!this.theme) {
                this.applyTheme(e.matches ? 'dark' : 'light');
            }
        });
    }

    applyTheme(theme) {
        const html = document.documentElement;

        if (theme === 'dark') {
            html.classList.add('dark-mode');
            html.classList.remove('light-mode');
        } else {
            html.classList.add('light-mode');
            html.classList.remove('dark-mode');
        }

        // 切换图片
        this.switchImage(theme);

        // 更新按钮图标
        this.updateToggleButton(theme);
    }


    switchImage(theme) {
        const imgDark = document.getElementById('img-dark');
        const imgLight = document.getElementById('img-light');

        if (!imgDark || !imgLight) return;

        // 根据主题调整两张图片的透明度
        if (theme === 'dark') {
            imgDark.style.opacity = '1';
            imgLight.style.opacity = '0';
        } else {
            imgDark.style.opacity = '0';
            imgLight.style.opacity = '1';
        }
    }





    toggle() {
        const currentTheme = document.documentElement.classList.contains('light-mode') ? 'light' : 'dark';
        const newTheme = currentTheme === 'light' ? 'dark' : 'light';

        this.theme = newTheme;
        this.saveTheme(newTheme);
        this.applyTheme(newTheme);

        // 显示提示
        this.showThemeToast(newTheme);
    }

    reset() {
        // 重置为跟随系统
        this.theme = null;
        this.saveTheme(null);
        window.__userThemePreference = null;
        this.followSystem();

        // 显示提示
        if (window.showToast) {
            window.showToast('Following system theme', 'Theme preference reset', 'success');
        }
    }

    createToggleButton() {
        // 创建按钮容器
        const buttonContainer = document.createElement('div');
        buttonContainer.className = 'fixed top-8 right-6 lg:right-12 z-30';
        buttonContainer.innerHTML = `
            <button 
                id="theme-toggle" 
                class="group relative p-2.5 rounded-xl bg-white/5 hover:bg-white/10 backdrop-blur-sm border border-white/10 transition-all duration-300 hover:scale-105 active:scale-95"
                aria-label="Toggle theme"
                title="Toggle theme"
            >
                <!-- 太阳图标 (白天模式) -->
                <svg id="sun-icon" class="w-5 h-5 text-yellow-400 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                </svg>
                
                <!-- 月亮图标 (黑夜模式) - 加深颜色 -->
                <svg id="moon-icon" class="w-5 h-5 text-indigo-300 transition-all duration-300 hidden" fill="currentColor" viewBox="0 0 24 24">
                    <path d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" />
                </svg>

                <!-- Tooltip -->
                <span class="absolute right-full mr-2 top-1/2 -translate-y-1/2 px-2 py-1 bg-gray-900 text-white text-xs rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none">
                    Switch theme
                </span>
            </button>
        `;

        document.body.appendChild(buttonContainer);

        // 绑定点击事件
        const button = document.getElementById('theme-toggle');
        button.addEventListener('click', () => this.toggle());

        // 长按重置为跟随系统（可选功能）
        let pressTimer;
        button.addEventListener('mousedown', () => {
            pressTimer = setTimeout(() => {
                this.reset();
                // 添加震动效果（如果支持）
                if (navigator.vibrate) {
                    navigator.vibrate(200);
                }
            }, 1000);
        });
        button.addEventListener('mouseup', () => clearTimeout(pressTimer));
        button.addEventListener('mouseleave', () => clearTimeout(pressTimer));
    }

    updateToggleButton(theme) {
        const sunIcon = document.getElementById('sun-icon');
        const moonIcon = document.getElementById('moon-icon');

        if (!sunIcon || !moonIcon) return;

        if (theme === 'dark') {
            // 显示太阳（因为点击后会切换到白天）
            sunIcon.classList.remove('hidden');
            moonIcon.classList.add('hidden');
        } else {
            // 显示月亮（因为点击后会切换到黑夜）
            sunIcon.classList.add('hidden');
            moonIcon.classList.remove('hidden');
        }
    }

    showThemeToast(theme) {
        if (window.showToast) {
            const message = theme === 'dark' ? 'Dark mode enabled' : 'Light mode enabled';
            const icon = theme === 'dark' ? '🌙' : '☀️';
            window.showToast(message, `${icon} Theme switched`, 'success');
        }
    }
}

// 页面加载时初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        window.themeManager = new ThemeManager();
    });
} else {
    window.themeManager = new ThemeManager();
}
