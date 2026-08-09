/*!
 * LiteZoom — 轻量图片灯箱(单文件,CSS 内嵌,零依赖)
 * https://litezoom.dev
 *
 * 两种模式:
 *   simple —— 点开放大 + 左右切换 + 计数器 + 关闭 + 键盘 + 下拉关闭
 *   full   —— 在 simple 基础上 + 右上角工具按钮(zoomIn/zoomOut/close)
 *             + 底部缩略图条 + caption + 滚轮/双指缩放 + 拖拽平移 pan + 拖拽下拉关闭
 *
 * 用法:
 *   LiteZoom.bind('.talk-images img', { mode: 'simple', group: fn });
 *   LiteZoom.bind('.post-content img, .page-content img', { mode: 'full', group: fn, caption: fn });
 *   // 也可手动打开:LiteZoom.open([{src,thumb,caption}], index, { mode });
 *
 * 委托式单一 document click 监听,动态插入图片后可调用 refresh/enhance。
 * 最后更新: 2026-08-09
 */
(function (window, document) {
    'use strict';

    if (window.LiteZoom) { return; }

    /* ----------------------------------------------------------------
     * 配置常量(可按需调整)
     * ---------------------------------------------------------------- */
    var MIN_SCALE = 1;
    var MAX_SCALE = 5;
    var ZOOM_STEP = 0.5;          // 工具按钮每次缩放步进
    var WHEEL_STEP = 0.0015;      // 滚轮缩放灵敏度
    var CLOSE_DRAG = 120;         // 下拉关闭阈值(px)
    var SWIPE_DRAG = 70;          // 左右切换阈值(px)
    var CLICK_SLOP = 8;           // 小于该位移视为点击而非拖拽(px)

    /* ----------------------------------------------------------------
     * 内嵌样式(注入一次)
     * ---------------------------------------------------------------- */
    var CSS = [
        '.litezoom{position:fixed;inset:0;z-index:2147483647;display:flex;align-items:center;justify-content:center;',
        'opacity:0;visibility:hidden;transition:opacity .28s ease,visibility .28s ease;',
        'touch-action:none;overscroll-behavior:contain;-webkit-user-select:none;user-select:none;--lz-fg:#fff;}',
        '.litezoom.is-open{opacity:1;visibility:visible;}',
        '.litezoom__backdrop{position:absolute;inset:0;background:rgba(17,24,39,.92);',
        '-webkit-backdrop-filter:blur(8px);backdrop-filter:blur(8px);}',
        '.litezoom__stage{position:relative;width:100%;height:100%;display:flex;align-items:center;justify-content:center;overflow:hidden;box-sizing:border-box;}',
        '.litezoom.has-thumbs .litezoom__stage{padding-bottom:148px;}',
        '.litezoom__img{max-width:92vw;max-height:88vh;object-fit:contain;transform-origin:center center;',
        'will-change:transform;-webkit-user-drag:none;border-radius:2px;box-shadow:0 8px 40px rgba(0,0,0,.45);',
        'opacity:0;cursor:zoom-out;}',
        '.litezoom.has-thumbs .litezoom__img{max-height:calc(100vh - 192px);}',
        '.litezoom__img.is-ready{opacity:1;}',
        '.litezoom[data-mode="full"] .litezoom__img{cursor:zoom-in;}',
        '.litezoom[data-mode="full"] .litezoom__img.is-zoomed{cursor:grab;}',
        '.litezoom__img.is-grabbing{cursor:grabbing!important;}',
        // 旋转加载指示
        '.litezoom__spinner{position:absolute;top:0;left:0;right:0;bottom:0;margin:auto;width:34px;height:34px;border-radius:50%;',
        'border:3px solid rgba(255,255,255,.25);border-top-color:#fff;animation:lz-spin .8s linear infinite;',
        'opacity:0;transition:opacity .2s;pointer-events:none;}',
        '.litezoom__spinner.is-active{opacity:1;}',
        '@keyframes lz-spin{to{transform:rotate(360deg);}}',
        // 顶部右侧工具栏
        '.litezoom__toolbar{position:absolute;top:14px;right:14px;display:flex;gap:8px;z-index:4;}',
        '.litezoom__btn{display:inline-flex;align-items:center;justify-content:center;width:40px;height:40px;',
        'border:none;border-radius:8px;background:rgba(255,255,255,.12);color:var(--lz-fg);cursor:pointer;',
        'transition:background .15s ease;-webkit-backdrop-filter:blur(4px);backdrop-filter:blur(4px);padding:0;}',
        '.litezoom__btn:hover{background:rgba(255,255,255,.24);}',
        '.litezoom__btn:active{background:rgba(255,255,255,.3);}',
        '.litezoom__btn svg{width:20px;height:20px;display:block;}',
        '.litezoom[data-mode="simple"] .litezoom__btn--zoom{display:none;}',
        // 顶部左侧计数器
        '.litezoom__counter{position:absolute;top:22px;left:20px;color:rgba(255,255,255,.85);font-size:14px;',
        'letter-spacing:.5px;z-index:4;font-variant-numeric:tabular-nums;}',
        '.litezoom.is-single .litezoom__counter{display:none;}',
        // 左右导航箭头
        '.litezoom__nav{position:absolute;top:50%;transform:translateY(-50%);width:44px;height:44px;border:none;',
        'border-radius:50%;background:rgba(255,255,255,.1);color:var(--lz-fg);cursor:pointer;display:flex;',
        'align-items:center;justify-content:center;z-index:4;transition:background .15s ease;padding:0;}',
        '.litezoom__nav:hover{background:rgba(255,255,255,.22);}',
        '.litezoom__nav--prev{left:16px;}',
        '.litezoom__nav--next{right:16px;}',
        '.litezoom__nav svg{width:24px;height:24px;display:block;}',
        '.litezoom.is-single .litezoom__nav{display:none;}',
        // caption 说明文字(full 模式):固定在图片底部说明区,避免跟随图片尺寸漂移
        '.litezoom__caption{position:absolute;left:50%;right:auto;bottom:92px;transform:translateX(-50%);max-width:min(620px,calc(100vw - 48px));',
        'padding:10px 14px;text-align:center;color:rgba(255,255,255,.94);font-size:14px;line-height:1.55;z-index:3;',
        'pointer-events:none;background:rgba(17,24,39,.58);border:1px solid rgba(255,255,255,.16);border-radius:0;',
        '-webkit-backdrop-filter:blur(14px) saturate(145%);backdrop-filter:blur(14px) saturate(145%);',
        'box-shadow:0 10px 28px rgba(0,0,0,.24);text-shadow:0 1px 2px rgba(0,0,0,.35);}',
        '.litezoom__caption:empty{display:none;}',
        '.litezoom[data-mode="simple"] .litezoom__caption{display:none;}',
        // 底部缩略图条(full 模式,多图)
        '.litezoom__thumbs{position:absolute;left:0;right:0;bottom:12px;display:flex;gap:8px;justify-content:center;align-items:center;',
        'height:64px;padding:0 14px;overflow-x:auto;z-index:4;scrollbar-width:none;background:linear-gradient(90deg,transparent,rgba(17,24,39,.58) 18%,rgba(17,24,39,.58) 82%,transparent);}',
        '.litezoom__thumbs::-webkit-scrollbar{display:none;}',
        '.litezoom__thumb{flex:0 0 auto;width:56px;height:56px;border:2px solid transparent;border-radius:6px;',
        'overflow:hidden;cursor:pointer;padding:0;background:rgba(255,255,255,.06);opacity:.5;',
        'transition:opacity .15s ease,border-color .15s ease;}',
        '.litezoom__thumb img{width:100%;height:100%;object-fit:cover;display:block;}',
        '.litezoom__thumb:hover{opacity:.85;}',
        '.litezoom__thumb.is-active{opacity:1;border-color:#fff;}',
        '.litezoom[data-mode="simple"] .litezoom__thumbs{display:none;}',
        '.litezoom.is-single .litezoom__thumbs{display:none;}',
        // 通用图片懒加载 / 淡入。站点可配合 image-loading-wrap 使用,外部单独引用本文件也可直接使用。
        '.litezoom-lazy{opacity:0;filter:blur(12px);transition:opacity .4s ease,filter .5s ease;}',
        '.litezoom-lazy.is-lz-loaded{opacity:1;filter:none;}',
        '.litezoom-lazy.is-lz-error{opacity:1;filter:none;}',
        // 不锁 overflow,保留系统滚动条 → 零重排零抖动(背景滚动由 wheel/touch/键盘 拦截)
        // 移动端微调
        '@media (max-width:640px){',
        '.litezoom__img{max-width:100vw;max-height:82vh;}',
        '.litezoom.has-thumbs .litezoom__stage{padding-bottom:132px;}',
        '.litezoom.has-thumbs .litezoom__img{max-height:calc(100vh - 168px);}',
        '.litezoom__nav{width:38px;height:38px;}',
        '.litezoom__thumb{width:48px;height:48px;}',
        '.litezoom__caption{bottom:78px;max-width:calc(100vw - 24px);font-size:13px;padding:8px 11px;border-radius:0;}',
        '}',
        '@media (prefers-reduced-motion:reduce){.litezoom-lazy{transition:none;filter:none;}'+'.litezoom{transition:none;}.litezoom__img{transition:none!important;}}'
    ].join('');

    var styleInjected = false;
    function injectStyle() {
        if (styleInjected) { return; }
        styleInjected = true;
        var style = document.createElement('style');
        style.id = 'litezoom-style';
        style.textContent = CSS;
        (document.head || document.documentElement).appendChild(style);
    }

    /* ----------------------------------------------------------------
     * 图标(内联 SVG,插件自包含)
     * ---------------------------------------------------------------- */
    // 可覆盖的无障碍文案:LiteZoom.labels({ viewer, prev, next, zoomIn, zoomOut, close, thumb })
    var LABELS = {
        viewer: '图片查看', prev: '上一张', next: '下一张',
        zoomIn: '放大', zoomOut: '缩小', close: '关闭',
        thumb: function (i) { return '第 ' + i + ' 张'; }
    };

    var ICON = {
        zoomIn: '<svg aria-hidden="true" focusable="false" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="m21.857 20.437l-5.23-5.22a8.27 8.27 0 1 0-1.41 1.41l5.22 5.23a1 1 0 0 0 1.42 0a1 1 0 0 0 0-1.42m-7.72-9.29h-3v3a1 1 0 1 1-2 0v-3h-3a1 1 0 1 1 0-2h3v-3a1 1 0 0 1 2 0v3h3a1 1 0 1 1 0 2"/></svg>',
        zoomOut: '<svg aria-hidden="true" focusable="false" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="m21.785 20.35l-5.22-5.22a8.18 8.18 0 1 0-1.41 1.42l5.22 5.22a1 1 0 1 0 1.41-1.42m-15.71-9.29a1 1 0 1 1 0-2h8a1 1 0 0 1 0 2z"/></svg>',
        close: '<svg aria-hidden="true" focusable="false" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>',
        prev: '<svg aria-hidden="true" focusable="false" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>',
        next: '<svg aria-hidden="true" focusable="false" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>'
    };

    /* ----------------------------------------------------------------
     * 工具函数
     * ---------------------------------------------------------------- */
    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }

    function isImageUrl(url) {
        return /\.(?:png|jpe?g|gif|webp|avif|svg|bmp)(?:[?#].*)?$/i.test(url || '');
    }

    function viewerSrc(img) {
        return img.currentSrc || img.getAttribute('src') || img.src || '';
    }

    // 取放大用的高清地址,优先级从高到低:
    //   1. <img data-litezoom="big.url"> —— 直接指定,无需链接包裹
    //   2. <a data-litezoom href="..."> —— 显式声明 href 就是图片,
    //      扩展名不再参与判断(现代 CDN 变换地址常无扩展名)
    //   3. <a href="大图.jpg"> —— 靠扩展名识别的启发式,历史行为
    //   4. 图片自身的 src
    function fullSrc(img) {
        var explicit = img.getAttribute('data-litezoom');
        if (explicit) { return explicit; }
        var link = img.closest ? img.closest('a[href]') : null;
        if (link && (link.hasAttribute('data-litezoom') || isImageUrl(link.getAttribute('href')))) {
            return link.href;
        }
        return viewerSrc(img);
    }

    /* ----------------------------------------------------------------
     * 灯箱单例(viewer)
     * ---------------------------------------------------------------- */
    var V = null;
    function viewer() { return V || (V = createViewer()); }

    function createViewer() {
        injectStyle();

        var el = document.createElement('div');
        el.className = 'litezoom';
        el.setAttribute('role', 'dialog');
        el.setAttribute('aria-modal', 'true');
        el.setAttribute('aria-label', LABELS.viewer);
        el.innerHTML =
            '<div class="litezoom__backdrop"></div>' +
            '<div class="litezoom__stage">' +
                '<div class="litezoom__spinner"></div>' +
                '<img class="litezoom__img" alt="">' +
            '</div>' +
            '<div class="litezoom__counter"></div>' +
            '<button type="button" class="litezoom__nav litezoom__nav--prev" aria-label="' + LABELS.prev + '">' + ICON.prev + '</button>' +
            '<button type="button" class="litezoom__nav litezoom__nav--next" aria-label="' + LABELS.next + '">' + ICON.next + '</button>' +
            '<div class="litezoom__toolbar">' +
                '<button type="button" class="litezoom__btn litezoom__btn--zoom" data-act="zoom-out" aria-label="' + LABELS.zoomOut + '">' + ICON.zoomOut + '</button>' +
                '<button type="button" class="litezoom__btn litezoom__btn--zoom" data-act="zoom-in" aria-label="' + LABELS.zoomIn + '">' + ICON.zoomIn + '</button>' +
                '<button type="button" class="litezoom__btn" data-act="close" aria-label="' + LABELS.close + '">' + ICON.close + '</button>' +
            '</div>' +
            '<div class="litezoom__caption"></div>' +
            '<div class="litezoom__thumbs"></div>';
        document.body.appendChild(el);

        var backdrop = el.querySelector('.litezoom__backdrop');
        var stage = el.querySelector('.litezoom__stage');
        var imgEl = el.querySelector('.litezoom__img');
        var spinner = el.querySelector('.litezoom__spinner');
        var counterEl = el.querySelector('.litezoom__counter');
        var prevBtn = el.querySelector('.litezoom__nav--prev');
        var nextBtn = el.querySelector('.litezoom__nav--next');
        var toolbar = el.querySelector('.litezoom__toolbar');
        var captionEl = el.querySelector('.litezoom__caption');
        var thumbsEl = el.querySelector('.litezoom__thumbs');

        // 运行时状态
        var items = [];
        var index = 0;
        var lastFocus = null;
        var mode = 'simple';
        var isOpen = false;
        var scale = 1, tx = 0, ty = 0;     // 图片 transform 状态
        var loadToken = 0;                  // 防止快速切图时旧图覆盖新图

        // 指针/手势状态
        var pointers = {};                  // pointerId -> {x,y}
        // 布局缓存:图片的无缩放基准尺寸与舞台尺寸。二者在一次手势期间
        // 都是常量,而 clampPan 跑在 pointermove/wheel 的每一帧上——
        // 不缓存的话每帧两次 getBoundingClientRect 强制布局。
        // 换图与窗口 resize 时失效,首次用到时惰性测量。
        var measure = null;
        var gesture = null;                 // 当前手势:'pan' | 'drag' | 'pinch' | null
        var startScale = 1, startDist = 0, pinchMid = { x: 0, y: 0 };
        var dragStart = { x: 0, y: 0 }, dragLast = { x: 0, y: 0 }, dragMoved = 0;

        /* ---------- transform ---------- */
        function applyTransform(animate) {
            imgEl.style.transition = animate ? 'transform .25s ease' : 'none';
            imgEl.style.transform = 'translate(' + tx + 'px,' + ty + 'px) scale(' + scale + ')';
            imgEl.classList.toggle('is-zoomed', scale > 1.001);
        }

        function resetTransform(animate) {
            scale = 1; tx = 0; ty = 0;
            applyTransform(!!animate);
        }

        // 以舞台某点 (cx,cy) 为锚点缩放到 next
        function zoomTo(next, cx, cy, animate) {
            next = clamp(next, MIN_SCALE, MAX_SCALE);
            if (next === scale) { return; }
            var rect = stage.getBoundingClientRect();
            var dx = (cx == null ? rect.left + rect.width / 2 : cx) - (rect.left + rect.width / 2);
            var dy = (cy == null ? rect.top + rect.height / 2 : cy) - (rect.top + rect.height / 2);
            var ratio = next / scale;
            tx = dx * (1 - ratio) + tx * ratio;
            ty = dy * (1 - ratio) + ty * ratio;
            scale = next;
            if (scale <= 1.001) { tx = 0; ty = 0; }
            clampPan();
            applyTransform(!!animate);
        }

        // 限制平移范围,避免把图片拖出视野太远
        function ensureMeasure() {
            if (measure) { return measure; }
            var rect = imgEl.getBoundingClientRect();
            var stageRect = stage.getBoundingClientRect();
            measure = {
                baseW: rect.width / scale, baseH: rect.height / scale,
                stageW: stageRect.width, stageH: stageRect.height
            };
            return measure;
        }
        function invalidateMeasure() { measure = null; }

        function clampPan() {
            if (scale <= 1) { tx = 0; ty = 0; return; }
            var m = ensureMeasure();
            var maxX = Math.max(0, (m.baseW * scale - m.stageW) / 2);
            var maxY = Math.max(0, (m.baseH * scale - m.stageH) / 2);
            tx = clamp(tx, -maxX, maxX);
            ty = clamp(ty, -maxY, maxY);
        }

        /* ---------- 渲染当前图 ---------- */
        function render() {
            var item = items[index];
            if (!item) { return; }
            resetTransform(false);

            var token = ++loadToken;
            spinner.classList.add('is-active');
            imgEl.classList.remove('is-ready');

            var loader = new Image();
            loader.onload = function () {
                if (token !== loadToken) { return; }
                imgEl.src = item.src;
                imgEl.alt = item.caption || '';
                imgEl.classList.add('is-ready');
                spinner.classList.remove('is-active');
                invalidateMeasure();
            };
            loader.onerror = function () {
                if (token !== loadToken) { return; }
                spinner.classList.remove('is-active');
            };
            loader.src = item.src;

            counterEl.textContent = (index + 1) + ' / ' + items.length;
            captionEl.textContent = item.caption || '';
            updateThumbsActive();
            preload(index + 1);
            preload(index - 1);
        }

        function preload(i) {
            if (i < 0 || i >= items.length) { return; }
            var im = new Image();
            im.src = items[i].src;
        }

        function buildThumbs() {
            thumbsEl.innerHTML = '';
            var multiple = items.length > 1;
            el.classList.toggle('has-thumbs', multiple && mode === 'full');
            if (mode !== 'full' || !multiple) { return; }
            items.forEach(function (item, i) {
                var btn = document.createElement('button');
                btn.type = 'button';
                btn.className = 'litezoom__thumb' + (i === index ? ' is-active' : '');
                btn.setAttribute('aria-label', LABELS.thumb(i + 1));
                var im = document.createElement('img');
                im.src = item.thumb || item.src;
                im.loading = 'lazy';
                btn.appendChild(im);
                btn.addEventListener('click', function (e) {
                    e.stopPropagation();
                    go(i);
                });
                thumbsEl.appendChild(btn);
            });
        }

        function updateThumbsActive() {
            var nodes = thumbsEl.children;
            for (var i = 0; i < nodes.length; i++) {
                nodes[i].classList.toggle('is-active', i === index);
            }
            var active = nodes[index];
            if (active && active.scrollIntoView) {
                active.scrollIntoView({ block: 'nearest', inline: 'center' });
            }
        }

        /* ---------- 切换 / 开关 ---------- */
        function go(i) {
            if (!items.length) { return; }
            index = (i + items.length) % items.length;
            render();
        }
        function next() { go(index + 1); }
        function prev() { go(index - 1); }

        function open(list, startIndex, opts) {
            opts = opts || {};
            items = list || [];
            if (!items.length) { return; }
            index = clamp(startIndex || 0, 0, items.length - 1);
            mode = opts.mode === 'full' ? 'full' : 'simple';
            el.setAttribute('data-mode', mode);
            el.classList.toggle('is-single', items.length <= 1);

            buildThumbs();
            render();

            isOpen = true;
            // 记录打开前的焦点,关闭时还原——否则 ESC 之后焦点掉回 body,
            // 键盘用户得从页面头部重新 Tab 一遍
            lastFocus = document.activeElement;
            // 强制重排后再加 is-open,触发淡入过渡
            void el.offsetWidth;
            el.classList.add('is-open');
            document.addEventListener('keydown', onKeydown, false);
            window.addEventListener('resize', invalidateMeasure);
            var closeBtn = el.querySelector('[data-act="close"]');
            if (closeBtn) { closeBtn.focus(); }
        }

        function close() {
            if (!isOpen) { return; }
            isOpen = false;
            // 作废在途加载:慢网络下用户等不及就关掉,迟到的 onload 不该
            // 把大图写回隐藏的 viewer(还会盖掉下面 300ms 后的清理)。
            loadToken++;
            el.classList.remove('is-open');
            backdrop.style.opacity = '';
            // 兜底清空手势状态:pointerup 被系统菜单/弹窗吞掉时,
            // 幽灵指针不能带进下一次打开
            pointers = {};
            gesture = null;
            document.removeEventListener('keydown', onKeydown, false);
            window.removeEventListener('resize', invalidateMeasure);
            if (lastFocus && lastFocus.focus) { try { lastFocus.focus(); } catch (err) {} }
            lastFocus = null;
            // 过渡结束后清理大图,释放内存
            window.setTimeout(function () {
                if (!isOpen) { imgEl.removeAttribute('src'); imgEl.classList.remove('is-ready'); }
            }, 300);
        }

        /* ---------- 键盘 ---------- */
        function onKeydown(e) {
            // 焦点陷阱:aria-modal 承诺了模态,Tab 就不能跑到灯箱后面的
            // 页面上去。在灯箱内部的可见按钮之间循环。
            if (e.key === 'Tab') {
                var focusables = el.querySelectorAll('button:not([style*="display: none"])');
                var list = [];
                for (var i = 0; i < focusables.length; i++) {
                    if (focusables[i].offsetParent !== null) { list.push(focusables[i]); }
                }
                if (!list.length) { e.preventDefault(); return; }
                var idx = list.indexOf(document.activeElement);
                var nextIdx = e.shiftKey
                    ? (idx <= 0 ? list.length - 1 : idx - 1)
                    : (idx === list.length - 1 || idx < 0 ? 0 : idx + 1);
                list[nextIdx].focus();
                e.preventDefault();
                return;
            }
            switch (e.key) {
                case 'Escape': close(); break;
                case 'ArrowLeft': prev(); break;
                case 'ArrowRight': next(); break;
                case '+': case '=': if (mode === 'full') { zoomTo(scale + ZOOM_STEP, null, null, true); } break;
                case '-': case '_': if (mode === 'full') { zoomTo(scale - ZOOM_STEP, null, null, true); } break;
                // 吞掉会滚动背景的按键(空格 / 翻页 / 上下方向 / Home / End)
                case ' ': case 'PageUp': case 'PageDown': case 'Home': case 'End': case 'ArrowUp': case 'ArrowDown': break;
                default: return;
            }
            e.preventDefault();
        }

        /* ---------- 工具按钮 / 导航 / 背景点击 ---------- */
        toolbar.addEventListener('click', function (e) {
            var btn = e.target.closest('[data-act]');
            if (!btn) { return; }
            e.stopPropagation();
            var act = btn.getAttribute('data-act');
            if (act === 'zoom-in') { zoomTo(scale + ZOOM_STEP, null, null, true); }
            else if (act === 'zoom-out') { zoomTo(scale - ZOOM_STEP, null, null, true); }
            else if (act === 'close') { close(); }
        });
        prevBtn.addEventListener('click', function (e) { e.stopPropagation(); prev(); });
        nextBtn.addEventListener('click', function (e) { e.stopPropagation(); next(); });

        /* ---------- 滚轮:full 缩放;两种模式都阻止背景滚动 ---------- */
        stage.addEventListener('wheel', function (e) {
            if (!isOpen) { return; }
            e.preventDefault(); // 灯箱打开时吞掉滚轮,背景不滚动
            if (mode === 'full') {
                zoomTo(scale * (1 - e.deltaY * WHEEL_STEP), e.clientX, e.clientY, false);
            }
        }, { passive: false });

        /* ---------- 指针手势:pan / pinch / 拖拽切换 / 下拉关闭 ---------- */
        function pointerCount() { return Object.keys(pointers).length; }

        stage.addEventListener('pointerdown', function (e) {
            // 只认主键(触屏与笔尖也是 0):右键要留给系统菜单,而且它的
            // pointerup 会被菜单吞掉——一旦入表就成幽灵指针,下一次左键
            // 拖拽会被误判成双指捏合,图片乱跳
            if (e.button !== 0) { return; }
            if (e.target.closest('.litezoom__thumbs, .litezoom__toolbar, .litezoom__nav')) { return; }
            pointers[e.pointerId] = { x: e.clientX, y: e.clientY };
            if (stage.setPointerCapture) { try { stage.setPointerCapture(e.pointerId); } catch (err) {} }

            if (pointerCount() === 2 && mode === 'full') {
                gesture = 'pinch';
                var pts = pointerList();
                startDist = distance(pts[0], pts[1]);
                startScale = scale;
                pinchMid = midpoint(pts[0], pts[1]);
            } else if (pointerCount() === 1) {
                gesture = (scale > 1.001 && mode === 'full') ? 'pan' : 'drag';
                dragStart = { x: e.clientX, y: e.clientY };
                dragLast = { x: e.clientX, y: e.clientY };
                dragMoved = 0;
                imgEl.style.transition = 'none';
                if (gesture === 'pan') { imgEl.classList.add('is-grabbing'); }
            }
        });

        stage.addEventListener('pointermove', function (e) {
            if (!pointers[e.pointerId]) { return; }
            pointers[e.pointerId] = { x: e.clientX, y: e.clientY };

            if (gesture === 'pinch' && pointerCount() >= 2) {
                var pts = pointerList();
                var dist = distance(pts[0], pts[1]);
                if (startDist > 0) {
                    zoomTo(startScale * (dist / startDist), pinchMid.x, pinchMid.y, false);
                }
                return;
            }
            if (gesture === 'pan') {
                var dx = e.clientX - dragLast.x;
                var dy = e.clientY - dragLast.y;
                dragLast = { x: e.clientX, y: e.clientY };
                tx += dx; ty += dy;
                dragMoved += Math.abs(dx) + Math.abs(dy);
                clampPan();
                applyTransform(false);
                return;
            }
            if (gesture === 'drag') {
                var totalX = e.clientX - dragStart.x;
                var totalY = e.clientY - dragStart.y;
                dragMoved = Math.max(dragMoved, Math.abs(totalX) + Math.abs(totalY));
                // 跟随手指:水平 + 垂直;下拉时淡化背景
                imgEl.style.transform = 'translate(' + totalX + 'px,' + totalY + 'px) scale(1)';
                if (totalY > 0) {
                    backdrop.style.opacity = String(clamp(1 - totalY / 400, 0.3, 1));
                }
            }
        });

        function endPointer(e) {
            if (!pointers[e.pointerId]) { return; }
            delete pointers[e.pointerId];

            if (gesture === 'pinch') {
                if (pointerCount() < 2) {
                    gesture = pointerCount() === 1 ? 'pan' : null;
                    if (scale < 1) { resetTransform(true); }
                }
                return;
            }
            if (gesture === 'pan') {
                imgEl.classList.remove('is-grabbing');
                gesture = null;
                return;
            }
            if (gesture === 'drag') {
                gesture = null;
                var totalX = e.clientX - dragStart.x;
                var totalY = e.clientY - dragStart.y;
                backdrop.style.opacity = '';

                if (dragMoved < CLICK_SLOP) {
                    resetTransform(false); // 清除拖拽中产生的微小位移
                    // 视为点击:点图片 → simple 关闭 / 点背景 → 关闭
                    if (e.target === imgEl) {
                        if (mode === 'simple') { close(); }
                    } else {
                        close();
                    }
                    return;
                }
                // 下拉关闭(垂直为主)
                if (totalY > CLOSE_DRAG && totalY > Math.abs(totalX)) { close(); return; }
                // 左右切换(水平为主)
                if (Math.abs(totalX) > SWIPE_DRAG && Math.abs(totalX) > Math.abs(totalY)) {
                    if (totalX < 0) { next(); } else { prev(); }
                    return;
                }
                // 否则回弹
                resetTransform(true);
            }
        }
        stage.addEventListener('pointerup', endPointer);
        stage.addEventListener('pointercancel', endPointer);

        // 双击/双指轻点:在 1x 与 2x 间切换(full)
        imgEl.addEventListener('dblclick', function (e) {
            if (mode !== 'full') { return; }
            e.preventDefault();
            if (scale > 1.001) { resetTransform(true); }
            else { zoomTo(2, e.clientX, e.clientY, true); }
        });

        function pointerList() {
            return Object.keys(pointers).map(function (k) { return pointers[k]; });
        }
        function distance(a, b) { return Math.hypot(a.x - b.x, a.y - b.y); }
        function midpoint(a, b) { return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 }; }

        return { open: open, close: close };
    }

    /* ----------------------------------------------------------------
     * 绑定层:委托式 click,按 selector + 分组打开
     * ---------------------------------------------------------------- */
    var bindings = [];
    var registered = {};
    var docBound = false;

    function bind(selector, opts) {
        opts = opts || {};
        var key = selector + '::' + (opts.mode || 'simple');
        if (registered[key]) { return; }
        registered[key] = true;
        bindings.push({
            selector: selector,
            mode: opts.mode === 'full' ? 'full' : 'simple',
            group: typeof opts.group === 'function' ? opts.group : null,
            caption: typeof opts.caption === 'function' ? opts.caption : null,
            exclude: typeof opts.exclude === 'function' ? opts.exclude : null
        });
        ensureDocHandler();
    }

    function ensureDocHandler() {
        if (docBound) { return; }
        docBound = true;
        document.addEventListener('click', onDocClick, false);
    }

    function matchBinding(img) {
        for (var i = 0; i < bindings.length; i++) {
            var b = bindings[i];
            if (img.matches && img.matches(b.selector)) {
                if (b.exclude && b.exclude(img)) { return null; }
                return b;
            }
        }
        return null;
    }

    function onDocClick(e) {
        if (e.defaultPrevented || e.button || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) { return; }
        var img = e.target && e.target.tagName === 'IMG'
            ? e.target
            : (e.target && e.target.closest ? e.target.closest('img') : null);
        if (!img) { return; }
        var b = matchBinding(img);
        if (!b) { return; }

        // 若图片被包在指向「另一个页面」的真实链接里,放行导航,不劫持。
        // data-litezoom(在链接或图片上)是作者的显式声明,跳过这层猜测。
        var link = img.closest('a[href]');
        var explicit = img.hasAttribute('data-litezoom') || (link && link.hasAttribute('data-litezoom'));
        if (link && !explicit) {
            var href = link.getAttribute('href') || '';
            if (href && href !== '#' && !isImageUrl(href) && href !== viewerSrc(img)) { return; }
        }

        e.preventDefault();
        openFromImage(img, b);
    }

    function openFromImage(img, b) {
        var key = b.group ? b.group(img) : ('__all__' + b.selector);
        var nodes = Array.prototype.slice.call(document.querySelectorAll(b.selector)).filter(function (n) {
            if (b.exclude && b.exclude(n)) { return false; }
            return b.group ? (b.group(n) === key) : true;
        });
        if (!nodes.length) { nodes = [img]; }
        var start = nodes.indexOf(img);
        if (start < 0) { start = 0; }
        var list = nodes.map(function (n) {
            return {
                src: fullSrc(n),
                thumb: viewerSrc(n),
                caption: b.caption ? (b.caption(n) || '') : ''
            };
        });
        viewer().open(list, start, { mode: b.mode });
    }

    /* ----------------------------------------------------------------
     * 图片增强层:懒加载 + 淡入 + 兼容站点现有 image-loading-wrap
     * ---------------------------------------------------------------- */
    var enhancers = [];
    var enhanceRegistered = {};
    var lazyObserver = null;
    var lazyObserverMargin = '';

    // 同步判断一张图的最终状态。complete 只说明「结束了」,不说明成败:
    // 失败的图同样 complete=true,靠 naturalWidth/Height 区分。
    // (极少数无固有尺寸的 SVG 两者皆 0,会被判成失败——图床场景全是位图,
    // 接受这个边角以换取失败图不再被标成 is-lz-loaded。)
    function imageState(img) {
        if (!img || !img.complete) { return 'pending'; }
        if (img.naturalWidth > 0 || img.naturalHeight > 0) { return 'loaded'; }
        return (img.currentSrc || img.getAttribute('src')) ? 'error' : 'pending';
    }

    function ensureImageAttrs(img) {
        if (!img.hasAttribute('loading')) {
            img.setAttribute('loading', 'lazy');
        }
        img.setAttribute('decoding', 'async');
    }

    function deferredSource(img) {
        return img.getAttribute('data-src') || img.getAttribute('data-lz-src') || '';
    }

    function loadDeferredImage(img) {
        var src = deferredSource(img);
        var srcset = img.getAttribute('data-srcset') || img.getAttribute('data-lz-srcset') || '';
        var sizes = img.getAttribute('data-sizes') || img.getAttribute('data-lz-sizes') || '';
        if (sizes && !img.getAttribute('sizes')) {
            img.setAttribute('sizes', sizes);
        }
        if (srcset && !img.getAttribute('srcset')) {
            img.setAttribute('srcset', srcset);
        }
        if (src && !img.getAttribute('src')) {
            img.setAttribute('src', src);
        }
    }

    function ensureLazyObserver(rootMargin) {
        rootMargin = rootMargin || '280px 0px';
        if (!('IntersectionObserver' in window)) { return null; }
        if (lazyObserver && lazyObserverMargin === rootMargin) { return lazyObserver; }
        lazyObserverMargin = rootMargin;
        lazyObserver = new IntersectionObserver(function (entries) {
            entries.forEach(function (entry) {
                if (!entry.isIntersecting) { return; }
                lazyObserver.unobserve(entry.target);
                loadDeferredImage(entry.target);
            });
        }, { rootMargin: rootMargin, threshold: 0.01 });
        return lazyObserver;
    }

    function resolveWrapper(img, opts) {
        if (opts.avatar || opts.wrap === false || !img.parentNode) { return null; }
        if (img.parentElement && img.parentElement.classList.contains('image-loading-wrap')) {
            return img.parentElement;
        }
        if (opts.wrap !== true) { return null; }
        var wrapper = document.createElement('span');
        wrapper.className = 'image-loading-wrap is-loading';
        img.parentNode.insertBefore(wrapper, img);
        wrapper.appendChild(img);
        return wrapper;
    }

    function finishEnhancedImage(img, wrapper, opts, error) {
        img.classList.remove('is-image-loading', 'is-avatar-loading');
        img.classList.add('is-lz-loaded');
        if (error) { img.classList.add('is-lz-error'); }
        if (opts.avatar) {
            img.classList.add('is-avatar-loaded');
        }
        if (wrapper) {
            wrapper.classList.remove('is-loading');
            wrapper.classList.add('is-loaded');
            if (error) { wrapper.classList.add('is-error'); }
        }
    }

    function enhanceImage(img, opts) {
        if (!img || img.nodeType !== 1 || img.dataset.litezoomImageReady === '1') { return; }
        img.dataset.litezoomImageReady = '1';
        opts = opts || {};
        injectStyle();
        ensureImageAttrs(img);

        var wrapper = resolveWrapper(img, opts);
        if (wrapper) {
            wrapper.classList.add('image-loading-wrap', 'is-loading');
        }
        img.classList.add('litezoom-lazy');
        if (opts.avatar) {
            img.classList.add('is-avatar-loading');
        } else {
            img.classList.add('is-image-loading');
        }

        img.addEventListener('load', function () {
            finishEnhancedImage(img, wrapper, opts, false);
        }, { once: true });
        img.addEventListener('error', function () {
            finishEnhancedImage(img, wrapper, opts, true);
        }, { once: true });

        if (deferredSource(img) && !img.getAttribute('src')) {
            var observer = ensureLazyObserver(opts.rootMargin);
            if (observer) { observer.observe(img); }
            else { loadDeferredImage(img); }
            return;
        }
        var state = imageState(img);
        if (state !== 'pending') {
            finishEnhancedImage(img, wrapper, opts, state === 'error');
        }
    }

    function applyEnhancer(root, enhancer) {
        root = root && root.querySelectorAll ? root : document;
        Array.prototype.slice.call(root.querySelectorAll(enhancer.selector)).forEach(function (img) {
            enhanceImage(img, enhancer.opts);
        });
    }

    function enhance(selector, opts) {
        opts = opts || {};
        var key = selector + '::' + (opts.avatar ? 'avatar' : 'image') + '::' + (opts.wrap === true ? 'wrap' : 'nowrap');
        if (!enhanceRegistered[key]) {
            enhanceRegistered[key] = true;
            enhancers.push({ selector: selector, opts: opts });
        }
        applyEnhancer(opts.root || document, { selector: selector, opts: opts });
        if (opts.zoom) {
            bind(selector, opts);
        }
    }

    function refresh(root) {
        enhancers.forEach(function (enhancer) {
            applyEnhancer(root || document, enhancer);
        });
    }

    function autoEnhanceMarkedImages() {
        var selector = 'img[data-lz-lazy], img[data-litezoom-lazy]';
        if (document.querySelector(selector)) {
            enhance(selector, { wrap: false });
        }
    }

    /* ----------------------------------------------------------------
     * 公开 API
     * ---------------------------------------------------------------- */
    window.LiteZoom = {
        labels: function (map) {
            for (var k in map) { if (k in LABELS) { LABELS[k] = map[k]; } }
        },
        bind: bind,
        enhance: enhance,
        refresh: refresh,
        open: function (list, index, opts) { viewer().open(list, index || 0, opts || {}); },
        close: function () { if (V) { V.close(); } }
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', autoEnhanceMarkedImages, { once: true });
    } else {
        autoEnhanceMarkedImages();
    }

})(window, document);
