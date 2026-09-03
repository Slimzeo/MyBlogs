(function () {
    'use strict';

    var sizeMessageType = 'myblog:html-size';
    var readyMessageType = 'myblog:html-ready';
    var measureType = 'myblog:measure-html';
    var viewportMessageType = 'myblog:html-viewport';
    var protocolVersion = 3;
    var desktopBreakpoint = 1024;
    var desktopCanvasWidth = 1280;
    var maximumCanvasWidth = 2560;
    var maximumContentHeight = 1000000;
    var listenerBound = false;
    var resizeBound = false;
    var scrollBound = false;
    var resizeFrame = 0;
    var viewportFrame = 0;

    function currentElements() {
        var frame = document.getElementById('html-article-frame');
        return {
            frame: frame,
            viewport: frame && frame.closest('.fluid-html-frame-viewport'),
            shell: frame && frame.closest('.fluid-html-frame-shell'),
            loader: frame && frame.closest('.fluid-html-frame-viewport') &&
                frame.closest('.fluid-html-frame-viewport').querySelector('.fluid-html-frame-loader'),
            main: document.querySelector('.fluid-html-post-main')
        };
    }

    function applyTheme(elements) {
        if (elements.main && elements.shell && elements.main.dataset.htmlThemeColor) {
            elements.shell.style.setProperty('--html-theme-color', elements.main.dataset.htmlThemeColor);
        }
    }

    function layoutFrame(elements) {
        if (!elements.frame || !elements.viewport) return;
        var availableWidth = elements.viewport.clientWidth;
        if (!availableWidth) return;
        var measuredWidth = Math.min(maximumCanvasWidth, Number(elements.frame.dataset.contentWidth || 0));
        var canvasWidth = window.innerWidth >= desktopBreakpoint
            ? Math.max(availableWidth, desktopCanvasWidth, measuredWidth)
            : availableWidth;
        var scale = Math.min(1, availableWidth / canvasWidth);
        var contentHeight = Number(elements.frame.dataset.contentHeight || 0);

        elements.frame.style.width = Math.ceil(canvasWidth) + 'px';
        elements.frame.style.transform = 'scale(' + scale + ')';
        elements.frame.dataset.scale = String(scale);
        if (contentHeight > 0) {
            elements.frame.style.height = Math.ceil(contentHeight) + 'px';
            if (!elements.shell || !elements.shell.classList.contains('is-loading')) {
                elements.viewport.style.height = Math.ceil(contentHeight * scale) + 'px';
            }
            elements.frame.classList.add('is-sized');
        }
        scheduleViewportReport();
    }

    function reportViewport(elements) {
        if (!elements.frame || !elements.frame.contentWindow) return;
        var scale = Number(elements.frame.dataset.scale || 1);
        if (!Number.isFinite(scale) || scale <= 0) scale = 1;
        var frameRect = elements.frame.getBoundingClientRect();
        elements.frame.contentWindow.postMessage({
            type: viewportMessageType,
            version: protocolVersion,
            top: Math.max(0, -frameRect.top / scale),
            height: window.innerHeight / scale
        }, '*');
    }

    function scheduleViewportReport() {
        cancelAnimationFrame(viewportFrame);
        viewportFrame = requestAnimationFrame(function () {
            reportViewport(currentElements());
        });
    }

    function applyReportedSize(elements, payload) {
        var height = Number(payload.height);
        if (!Number.isFinite(height) || height < 1) return false;
        elements.frame.dataset.contentHeight = String(Math.min(maximumContentHeight, Math.ceil(height)));
        var reportedWidth = Number(payload.width);
        if (Number.isFinite(reportedWidth) && reportedWidth > 0) {
            elements.frame.dataset.contentWidth = String(Math.min(maximumCanvasWidth, Math.ceil(reportedWidth)));
        }
        layoutFrame(elements);
        return true;
    }

    function revealFrame(elements) {
        if (!elements.frame || !elements.viewport || !elements.shell ||
            elements.frame !== currentElements().frame ||
            elements.frame.dataset.htmlReady === '1') return;
        elements.frame.dataset.htmlReady = '1';
        window.clearTimeout(elements.frame.htmlRevealTimer);
        elements.shell.classList.remove('is-loading');
        elements.shell.setAttribute('aria-busy', 'false');
        elements.frame.removeAttribute('aria-hidden');
        elements.frame.removeAttribute('tabindex');
        if (elements.loader) elements.loader.setAttribute('aria-hidden', 'true');
        layoutFrame(elements);
    }

    function requestMeasurement(elements) {
        if (elements.frame && elements.frame.contentWindow) {
            elements.frame.contentWindow.postMessage({type: measureType, version: protocolVersion}, '*');
            scheduleViewportReport();
        }
    }

    function bindGlobalListeners() {
        if (!listenerBound) {
            listenerBound = true;
            window.addEventListener('message', function (event) {
                var elements = currentElements();
                var payload = event.data;
                if (!elements.frame || event.source !== elements.frame.contentWindow ||
                    !payload || payload.version !== protocolVersion ||
                    (payload.type !== sizeMessageType && payload.type !== readyMessageType)) return;
                applyReportedSize(elements, payload);
                if (payload.type === readyMessageType) revealFrame(elements);
            });
        }
        if (!resizeBound) {
            resizeBound = true;
            window.addEventListener('resize', function () {
                cancelAnimationFrame(resizeFrame);
                resizeFrame = requestAnimationFrame(function () {
                    var elements = currentElements();
                    layoutFrame(elements);
                    requestMeasurement(elements);
                });
            }, {passive: true});
        }
        if (!scrollBound) {
            scrollBound = true;
            window.addEventListener('scroll', scheduleViewportReport, {passive: true});
        }
    }

    function mount() {
        var elements = currentElements();
        bindGlobalListeners();
        applyTheme(elements);
        if (elements.frame && elements.shell && elements.frame.dataset.htmlReady !== '1') {
            elements.shell.classList.add('is-loading');
            elements.shell.setAttribute('aria-busy', 'true');
            elements.frame.setAttribute('aria-hidden', 'true');
            elements.frame.setAttribute('tabindex', '-1');
            if (elements.loader) elements.loader.removeAttribute('aria-hidden');
        }
        layoutFrame(elements);
        if (elements.frame && !elements.frame.dataset.htmlFrameBound) {
            elements.frame.dataset.htmlFrameBound = '1';
            elements.frame.addEventListener('load', function () {
                requestMeasurement(currentElements());
            });
            elements.frame.htmlRevealTimer = window.setTimeout(function () {
                revealFrame(elements);
            }, 10000);
        }
        requestMeasurement(elements);
    }

    window.HTMLArticleFrame = {mount: mount};
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', mount, {once: true});
    } else {
        mount();
    }
})();
