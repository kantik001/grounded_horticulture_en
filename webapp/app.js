const tg = window.Telegram && window.Telegram.WebApp;
        if (tg) {
            tg.ready();
            tg.expand();
            document.documentElement.style.setProperty('--tg-theme-bg-color', tg.backgroundColor || '#e5ddd5');
            document.documentElement.style.setProperty('--tg-theme-text-color', tg.textColor || '#111');
            document.documentElement.style.setProperty('--tg-theme-hint-color', tg.hintColor || '#706f6f');
            document.documentElement.style.setProperty('--tg-theme-button-color', tg.buttonColor || '#2aabee');
            document.documentElement.style.setProperty('--tg-theme-button-text-color', tg.buttonTextColor || '#fff');
            document.documentElement.style.setProperty('--tg-theme-secondary-bg-color', tg.secondaryBackgroundColor || '#fff');
            if (tg.themeParams && tg.themeParams.section_header_text_color) {
                document.documentElement.style.setProperty('--tg-theme-header-text-color', tg.themeParams.section_header_text_color);
            }
            if (tg.themeParams && tg.themeParams.section_bg_color) {
                document.documentElement.style.setProperty('--tg-theme-header-bg-color', tg.themeParams.section_bg_color);
            }
            if (tg.MainButton) tg.MainButton.hide();
        }

        const STORAGE_KEY = 'apple_gardener_session_id';
        const CROP_STORAGE_KEY = 'apple_gardener_crop_id';
        const API_KEY_STORAGE_KEY = 'apple_gardener_api_key';
        const API_BASE_STORAGE_KEY = 'apple_gardener_api_base';
        const API_BASE_SCHEMA_VERSION = '2';

        var authInfo = null;
        var webLoginResolver = null;
        // True when running inside Telegram with signed initData.
        function isTelegramClient() {
            return !!(tg && tg.initData);
        }

        // Returns the browser API key from sessionStorage, or ''.
        function getStoredApiKey() {
            return sessionStorage.getItem(API_KEY_STORAGE_KEY) || '';
        }

        // Saves or clears the browser API key in sessionStorage.
        function setStoredApiKey(key) {
            if (key) {
                sessionStorage.setItem(API_KEY_STORAGE_KEY, key);
            } else {
                sessionStorage.removeItem(API_KEY_STORAGE_KEY);
            }
        }

        if (sessionStorage.getItem('apple_gardener_api_base_v') !== API_BASE_SCHEMA_VERSION) {
            sessionStorage.removeItem(API_BASE_STORAGE_KEY);
            sessionStorage.setItem('apple_gardener_api_base_v', API_BASE_SCHEMA_VERSION);
        }

        let apiBaseUrl = sessionStorage.getItem(API_BASE_STORAGE_KEY) || '/api/';

        let sessionId = null;
        let cropId = sessionStorage.getItem(CROP_STORAGE_KEY) || 'apple';
        let pendingFile = null;
        let pendingObjectUrl = null;
        let sending = false;

        const el = {
            messagesRoot: document.getElementById('messagesRoot'),
            chatScroll: document.getElementById('chatScroll'),
            inputText: document.getElementById('inputText'),
            sendBtn: document.getElementById('sendBtn'),
            fileInput: document.getElementById('fileInput'),
            attachBtn: document.getElementById('attachBtn'),
            attachmentStrip: document.getElementById('attachmentStrip'),
            attachmentThumb: document.getElementById('attachmentThumb'),
            clearAttachment: document.getElementById('clearAttachment'),
            photoBetaNotice: document.getElementById('photoBetaNotice'),
            typingLine: document.getElementById('typingLine'),
            toast: document.getElementById('toast'),
            cropSelect: document.getElementById('cropSelect'),
            onboardingRoot: document.getElementById('onboardingRoot'),
            onboardingChips: document.getElementById('onboardingChips'),
            headerTitle: document.getElementById('headerTitle'),
            headerSubtitle: document.getElementById('headerSubtitle'),
            cropLabel: document.getElementById('cropLabel'),
            headerDisclaimer: document.getElementById('headerDisclaimer'),
            onboardingTitle: document.getElementById('onboardingTitle'),
            chatDivider: document.getElementById('chatDivider'),
            webLoginOverlay: document.getElementById('webLoginOverlay'),
            webLoginTitle: document.getElementById('webLoginTitle'),
            webLoginHint: document.getElementById('webLoginHint'),
            webLoginKey: document.getElementById('webLoginKey'),
            webLoginSubmit: document.getElementById('webLoginSubmit'),
            webLoginNote: document.getElementById('webLoginNote'),
            webLoginError: document.getElementById('webLoginError'),
        };

        // Fills the web login overlay with English UI text.
        function applyWebLoginCopy() {
            if (!el.webLoginTitle) return;
            el.webLoginTitle.textContent = 'Sign in';
            el.webLoginHint.textContent = 'Enter your access key to use the assistant in the browser.';
            el.webLoginKey.placeholder = 'Access key';
            el.webLoginSubmit.textContent = 'Continue';
            el.webLoginNote.textContent = 'Telegram users: open the app from the bot.';
        }

        // Loads branding config and applies titles, labels and disclaimer.
        async function loadBranding() {
            try {
                var res = await apiFetch('/branding', { method: 'GET' });
                var data = parseApiResponseJson(await res.text());
                if (!data.success || !data.branding) return;
                var b = data.branding;
                if (el.headerTitle && b.app_title) {
                    el.headerTitle.textContent = (b.header_emoji ? b.header_emoji + ' ' : '') + b.app_title;
                }
                if (el.headerSubtitle && b.header_subtitle) el.headerSubtitle.textContent = b.header_subtitle;
                if (el.cropLabel && b.crop_label) el.cropLabel.textContent = b.crop_label;
                if (el.headerDisclaimer && b.disclaimer) el.headerDisclaimer.textContent = b.disclaimer;
                if (el.onboardingTitle && b.onboarding_title) el.onboardingTitle.textContent = b.onboarding_title;
                if (el.chatDivider && b.chat_divider) el.chatDivider.textContent = b.chat_divider;
                if (el.photoBetaNotice && b.photo_beta_notice) el.photoBetaNotice.textContent = b.photo_beta_notice;
                if (b.app_title) document.title = b.app_title + ' — chat';
            } catch (e) {
                console.warn('loadBranding', e);
            }
        }

        // Shows a toast message that auto-hides after a few seconds.
        function showToast(msg) {
            el.toast.textContent = msg;
            el.toast.classList.add('show');
            clearTimeout(showToast._t);
            showToast._t = setTimeout(function() { el.toast.classList.remove('show'); }, 4200);
        }

        /** Telegram initData — cryptographically signed user payload (see core.telegram.org/bots/webapps). */
        function getTelegramInitData() {
            if (tg && tg.initData) {
                return String(tg.initData);
            }
            return '';
        }

        /** API headers: Telegram initData or browser API key. */
        function withAuthHeaders(extra) {
            var h = Object.assign({}, extra || {});
            var initData = getTelegramInitData();
            if (initData) {
                h['X-Telegram-Init-Data'] = initData;
            } else {
                var apiKey = getStoredApiKey();
                if (apiKey) {
                    h['X-API-Key'] = apiKey;
                }
            }
            return h;
        }

        // Queries /auth/info across API base candidates to detect the auth mode.
        async function fetchAuthInfo() {
            var candidates = buildApiCandidates();
            for (var i = 0; i < candidates.length; i++) {
                var base = candidates[i];
                var baseNorm = base.endsWith('/') ? base : base + '/';
                var url = baseNorm + 'auth/info';
                try {
                    var res = await fetch(url, { method: 'GET' });
                    var txt = await res.text();
                    if (!isOurAPIJsonBody(txt)) continue;
                    var data = JSON.parse(txt);
                    if (data.success && data.auth) {
                        return data.auth;
                    }
                } catch (e) {
                    console.warn('fetchAuthInfo', url, e);
                }
            }
            return null;
        }

        // Opens the login overlay and returns a promise resolved on successful sign-in.
        function showWebLoginOverlay() {
            applyWebLoginCopy();
            el.webLoginError.hidden = true;
            el.webLoginKey.value = '';
            el.webLoginOverlay.hidden = false;
            el.webLoginKey.focus();
            return new Promise(function(resolve, reject) {
                webLoginResolver = { resolve: resolve, reject: reject };
            });
        }

        // Hides the web login overlay.
        function hideWebLoginOverlay() {
            el.webLoginOverlay.hidden = true;
        }

        // Verifies an API key by creating a session; restores the previous key on failure.
        async function validateApiKey(key) {
            var prev = getStoredApiKey();
            setStoredApiKey(key);
            try {
                var res = await apiFetch('/session', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json; charset=utf-8' },
                    body: JSON.stringify({ crop_id: cropId })
                });
                var data = parseApiResponseJson(await res.text());
                if (res.ok && data.session_id) {
                    sessionId = data.session_id;
                    sessionStorage.setItem(STORAGE_KEY, sessionId);
                    return true;
                }
                setStoredApiKey(prev);
                throw new Error(data.error || 'Invalid access key');
            } catch (e) {
                setStoredApiKey(prev);
                throw e;
            }
        }

        // Ensures browser clients are authenticated, prompting for a key when required.
        async function ensureWebAuth() {
            if (isTelegramClient()) return;
            if (!authInfo) {
                authInfo = await fetchAuthInfo();
            }
            if (authInfo && authInfo.dev_mode) return;
            if (getStoredApiKey()) return;
            if (authInfo && authInfo.web_api_key) {
                await showWebLoginOverlay();
                return;
            }
            if (authInfo && authInfo.telegram) {
                throw new Error('Open this app from the Telegram bot.');
            }
            throw new Error('Server auth is not configured.');
        }

        if (el.webLoginSubmit) {
            el.webLoginSubmit.addEventListener('click', function() {
                var key = (el.webLoginKey.value || '').trim();
                if (!key) {
                    el.webLoginError.textContent = 'Enter a key';
                    el.webLoginError.hidden = false;
                    return;
                }
                el.webLoginSubmit.disabled = true;
                validateApiKey(key).then(function() {
                    hideWebLoginOverlay();
                    if (webLoginResolver) {
                        webLoginResolver.resolve();
                        webLoginResolver = null;
                    }
                }).catch(function(e) {
                    el.webLoginError.textContent = e.message || 'Error';
                    el.webLoginError.hidden = false;
                }).finally(function() {
                    el.webLoginSubmit.disabled = false;
                });
            });
            el.webLoginKey.addEventListener('keydown', function(e) {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    el.webLoginSubmit.click();
                }
            });
        }

        // Removes empty and duplicate entries from a list of API base URLs.
        function dedupeApiBases(list) {
            var out = [];
            var seen = {};
            for (var i = 0; i < list.length; i++) {
                var b = list[i];
                if (!b || seen[b]) continue;
                seen[b] = true;
                out.push(b);
            }
            return out;
        }

        /** Direct Go on port 8080 (bypass nginx proxy). */
        function alternateApiBase8080() {
            try {
                var p = window.location.protocol;
                var h = window.location.hostname;
                if (!h) return null;
                if (String(window.location.port) === '8080') return null;
                var bases = [];
                bases.push('http://127.0.0.1:8080/api/');
                if (h !== '127.0.0.1') {
                    bases.push(p + '//' + h + ':8080/api/');
                }
                return bases;
            } catch (e) {
                return null;
            }
        }

        /** Only our Go API responses include a success field; foreign JSON broke URL fallback. */
        function isOurAPIJsonBody(txt) {
            var t = String(txt).trim();
            if (t.charAt(0) !== '{') return false;
            try {
                var o = JSON.parse(t);
                if (!o || typeof o !== 'object') return false;
                return Object.prototype.hasOwnProperty.call(o, 'success');
            } catch (e) {
                return false;
            }
        }

        // Builds the ordered, deduplicated list of API base URLs to try.
        function buildApiCandidates() {
            var port = String(window.location.port || '');
            var list = [];
            list.push(apiBaseUrl);
            list.push('/api/');
            var alts = alternateApiBase8080();
            if (alts) {
                for (var a = 0; a < alts.length; a++) {
                    list.push(alts[a]);
                }
            }
            return dedupeApiBases(list);
        }

        /**
         * API request: same origin (/api/) first, then Go on :8080.
         * path has a leading slash, e.g. "/session" -> /api/session.
         */
        async function apiFetch(path, init) {
            var candidates = buildApiCandidates();
            var lastRes = null;
            for (var i = 0; i < candidates.length; i++) {
                var base = candidates[i];
                var baseNorm = base.endsWith('/') ? base : base + '/';
                var pathNorm = String(path).replace(/^\//, '');
                var url = baseNorm + pathNorm;
                var res;
                try {
                    var opts = init ? Object.assign({}, init) : {};
                    opts.headers = withAuthHeaders(opts.headers);
                    if (!opts.signal && url.indexOf(':8080') !== -1 &&
                        typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
                        opts.signal = AbortSignal.timeout(5000);
                    }
                    res = await fetch(url, opts);
                } catch (e) {
                    continue;
                }
                lastRes = res;
                var peek = await res.clone().text();
                if (res.ok || isOurAPIJsonBody(peek)) {
                    if (i > 0) {
                        apiBaseUrl = baseNorm;
                        sessionStorage.setItem(API_BASE_STORAGE_KEY, apiBaseUrl);
                    }
                    return res;
                }
            }
            if (!lastRes) {
                throw new Error('Cannot reach the API. Start docker compose (webapp + server) or Go on port 8080.');
            }
            return lastRes;
        }

        /** SSE endpoint request (no JSON success check on body). */
        async function apiStreamFetch(path, init) {
            var candidates = buildApiCandidates();
            var lastRes = null;
            for (var i = 0; i < candidates.length; i++) {
                var base = candidates[i];
                var baseNorm = base.endsWith('/') ? base : base + '/';
                var pathNorm = String(path).replace(/^\//, '');
                var url = baseNorm + pathNorm;
                var res;
                try {
                    var opts = init ? Object.assign({}, init) : {};
                    opts.headers = withAuthHeaders(Object.assign({
                        'Accept': 'text/event-stream'
                    }, opts.headers || {}));
                    res = await fetch(url, opts);
                } catch (e) {
                    continue;
                }
                lastRes = res;
                var ct = (res.headers.get('content-type') || '').toLowerCase();
                if (res.ok || ct.indexOf('text/event-stream') !== -1) {
                    if (i > 0) {
                        apiBaseUrl = baseNorm;
                        sessionStorage.setItem(API_BASE_STORAGE_KEY, apiBaseUrl);
                    }
                    return res;
                }
                var peek = await res.clone().text();
                if (isOurAPIJsonBody(peek)) {
                    if (i > 0) {
                        apiBaseUrl = baseNorm;
                        sessionStorage.setItem(API_BASE_STORAGE_KEY, apiBaseUrl);
                    }
                    return res;
                }
            }
            if (!lastRes) {
                throw new Error('Cannot reach the API. Start docker compose (webapp + server) or Go on port 8080.');
            }
            return lastRes;
        }

        // Reads the SSE stream and dispatches parsed events to handlers.
        function readSSEStream(res, handlers) {
            if (!res.body || !res.body.getReader) {
                return Promise.reject(new Error('Streaming is not supported by this browser'));
            }
            var reader = res.body.getReader();
            var decoder = new TextDecoder();
            var buffer = '';
            // Parses one SSE block into event name and JSON payload, then calls its handler.
            function parseBlock(block) {
                var eventName = 'message';
                var dataLine = '';
                block.split('\n').forEach(function(line) {
                    if (line.indexOf('event:') === 0) eventName = line.slice(6).trim();
                    else if (line.indexOf('data:') === 0) dataLine = line.slice(5).trim();
                });
                if (!dataLine) return;
                var payload;
                try { payload = JSON.parse(dataLine); } catch (e) { return; }
                if (handlers[eventName]) handlers[eventName](payload);
            }
            // Reads stream chunks recursively, splitting the buffer into SSE blocks.
            function pump() {
                return reader.read().then(function(chunk) {
                    if (chunk.done) {
                        if (buffer.trim()) parseBlock(buffer);
                        return;
                    }
                    buffer += decoder.decode(chunk.value, { stream: true });
                    var parts = buffer.split('\n\n');
                    buffer = parts.pop() || '';
                    parts.forEach(parseBlock);
                    return pump();
                });
            }
            return pump();
        }

        // Renders a streamed assistant reply, updating the chat as SSE events arrive.
        async function consumeMessageStream(res, userText) {
            clearChatHintIfEmpty();
            var userRow = buildMessageRow({ role: 'user', content: userText });
            var asstRow = buildMessageRow({ role: 'assistant', content: '' });
            el.messagesRoot.appendChild(userRow);
            el.messagesRoot.appendChild(asstRow);
            var asstBody = asstRow.querySelector('.body');
            if (!asstBody) {
                asstBody = document.createElement('div');
                asstBody.className = 'body';
                asstRow.querySelector('.bubble').appendChild(asstBody);
            }
            el.typingLine.classList.remove('active');
            updateOnboardingVisibility();
            scrollToBottom();

            await readSSEStream(res, {
                meta: function(data) {
                    if (data.session_id) {
                        sessionId = data.session_id;
                        sessionStorage.setItem(STORAGE_KEY, sessionId);
                    }
                    if (data.crop_id) {
                        cropId = data.crop_id;
                        sessionStorage.setItem(CROP_STORAGE_KEY, cropId);
                        el.cropSelect.value = cropId;
                    }
                    if (data.user_message) {
                        var newUserRow = buildMessageRow(data.user_message);
                        el.messagesRoot.replaceChild(newUserRow, userRow);
                        userRow = newUserRow;
                    }
                },
                delta: function(data) {
                    if (data.content) {
                        asstBody.textContent += data.content;
                        scrollToBottom();
                    }
                },
                done: function(data) {
                    if (data.assistant_message) {
                        var finalRow = buildMessageRow(data.assistant_message);
                        el.messagesRoot.replaceChild(finalRow, asstRow);
                    }
                    updateOnboardingVisibility();
                    scrollToBottom();
                },
                error: function(data) {
                    showToast(data.error || 'Stream error');
                }
            });
        }

        /**
         * Parse a JSON object from the response body. Gin 404 returns "404 page not found" —
         * JSON.parse then reads 404 as a number and fails at position 4 ('p' in 'page').
         */
        function parseApiResponseJson(raw) {
            var s = String(raw).replace(/^\uFEFF/, '').trim();
            if (!s) {
                throw new Error('Empty server response');
            }
            if (s.indexOf('404 page not found') === 0 || /^404\s/.test(s)) {
                throw new Error('API route not found (404). Restart containers: docker compose up --build');
            }
            if (s.charAt(0) === '<') {
                throw new Error('Expected JSON but got HTML — check proxy and API URL.');
            }
            var i = s.indexOf('{');
            if (i < 0) {
                throw new Error('No JSON in response: ' + s.slice(0, 200));
            }
            var depth = 0;
            var inStr = false;
            var esc = false;
            for (var j = i; j < s.length; j++) {
                var c = s[j];
                if (inStr) {
                    if (esc) {
                        esc = false;
                        continue;
                    }
                    if (c === '\\') {
                        esc = true;
                        continue;
                    }
                    if (c === '"') {
                        inStr = false;
                    }
                    continue;
                }
                if (c === '"') {
                    inStr = true;
                    continue;
                }
                if (c === '{') {
                    depth++;
                } else if (c === '}') {
                    depth--;
                    if (depth === 0) {
                        return JSON.parse(s.slice(i, j + 1));
                    }
                }
            }
            throw new Error('Incomplete JSON in server response');
        }

        // Maps a classifier label to a human-readable disease name.
        function formatPredictionName(prediction) {
            const names = {
                healthy_apple: 'Healthy apple',
                apple_scab: 'Apple scab',
                black_rot: 'Black rot',
                cedar_apple_rust: 'Cedar apple rust',
                healthy_leaf: 'Healthy leaf',
                powdery_mildew: 'Powdery mildew',
                fire_blight: 'Fire blight',
                bitter_rot: 'Bitter rot',
                blue_mold: 'Blue mold',
                brown_rot: 'Brown rot'
            };
            return names[prediction] || (prediction || '').replace(/_/g, ' ');
        }

        // Loads onboarding questions for the crop and renders them as clickable chips.
        async function loadOnboarding(selectedCrop) {
            try {
                var res = await apiFetch('/onboarding?crop_id=' + encodeURIComponent(selectedCrop || cropId), { method: 'GET' });
                var data = parseApiResponseJson(await res.text());
                var questions = (data.success && data.questions) ? data.questions : [];
                el.onboardingChips.innerHTML = '';
                if (!questions.length) {
                    el.onboardingRoot.hidden = true;
                    return;
                }
                questions.forEach(function(q) {
                    var btn = document.createElement('button');
                    btn.type = 'button';
                    btn.className = 'onboarding-chip';
                    btn.textContent = q;
                    btn.addEventListener('click', function() {
                        el.inputText.value = q;
                        autoResize();
                        sendMessage();
                    });
                    el.onboardingChips.appendChild(btn);
                });
                updateOnboardingVisibility();
            } catch (e) {
                console.error('loadOnboarding', e);
                el.onboardingRoot.hidden = true;
            }
        }

        // Shows onboarding chips only when the chat is empty.
        function updateOnboardingVisibility() {
            var hasMessages = el.messagesRoot.querySelector('.row');
            el.onboardingRoot.hidden = !el.onboardingChips.children.length || !!hasMessages;
        }

        // Sends a thumbs up/down rating for an assistant message and locks the buttons.
        async function sendFeedback(messageId, rating) {
            if (!sessionId || !messageId) return;
            try {
                var res = await apiFetch('/feedback', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json; charset=utf-8' },
                    body: JSON.stringify({ session_id: sessionId, message_id: messageId, rating: rating })
                });
                var data = parseApiResponseJson(await res.text());
                if (!res.ok || !data.success) {
                    showToast(data.error || 'Could not save rating');
                    return;
                }
                var btn = el.messagesRoot.querySelector('[data-feedback-for="' + messageId + '"][data-rating="' + rating + '"]');
                if (btn && btn.parentElement) {
                    btn.parentElement.querySelectorAll('.feedback-btn').forEach(function(b) {
                        b.classList.toggle('active', Number(b.getAttribute('data-rating')) === rating);
                        b.disabled = true;
                    });
                }
            } catch (e) {
                showToast(e.message || 'Rating error');
            }
        }

        // Loads the crops catalog and fills the crop selector.
        async function loadCropsCatalog() {
            try {
                var res = await apiFetch('/crops', { method: 'GET' });
                var data = parseApiResponseJson(await res.text());
                if (!data.success || !data.crops) return;
                el.cropSelect.innerHTML = '';
                data.crops.forEach(function(c) {
                    var opt = document.createElement('option');
                    opt.value = c.id;
                    var label = (c.emoji ? c.emoji + ' ' : '') + (c.name_en || c.name_ru);
                    if (!c.rag_enabled && !c.cv_enabled) label += ' (soon)';
                    opt.textContent = label;
                    el.cropSelect.appendChild(opt);
                });
                cropId = sessionStorage.getItem(CROP_STORAGE_KEY) || data.default_crop || 'apple';
                el.cropSelect.value = cropId;
            } catch (e) {
                console.error('loadCropsCatalog', e);
            }
        }

        // Creates a fresh session for the selected crop and resets the chat.
        async function createSessionWithCrop(selectedCrop) {
            cropId = selectedCrop;
            sessionStorage.setItem(CROP_STORAGE_KEY, cropId);
            sessionStorage.removeItem(STORAGE_KEY);
            sessionId = null;
            var res = await apiFetch('/session', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json; charset=utf-8' },
                body: JSON.stringify({ crop_id: cropId })
            });
            var data = parseApiResponseJson(await res.text());
            if (!res.ok || !data.session_id) {
                throw new Error(data.error || 'Could not create session');
            }
            sessionId = data.session_id;
            if (data.crop_id) cropId = data.crop_id;
            sessionStorage.setItem(STORAGE_KEY, sessionId);
            renderMessages([]);
            loadOnboarding(cropId);
        }

        el.cropSelect.addEventListener('change', function() {
            var next = el.cropSelect.value;
            if (next === cropId && sessionId) return;
            createSessionWithCrop(next).catch(function(e) {
                showToast(e.message || 'Crop switch error');
                el.cropSelect.value = cropId;
            });
        });

        // Scrolls the chat to the newest message on the next frame.
        function scrollToBottom() {
            requestAnimationFrame(function() {
                el.chatScroll.scrollTop = el.chatScroll.scrollHeight;
            });
        }

        /** Server photo: fetch with initData -> blob URL (img tags do not send auth). */
        async function loadAuthedImage(imgEl, imagePath) {
            try {
                var path = String(imagePath || '').replace(/^\/api\//, '');
                if (path.charAt(0) === '/') path = path.slice(1);
                var res = await apiFetch(path, { method: 'GET' });
                if (!res.ok) return;
                var blob = await res.blob();
                imgEl.src = URL.createObjectURL(blob);
            } catch (e) {
                console.error('loadAuthedImage', e);
            }
        }

        // Removes the placeholder hint when the chat has no messages yet.
        function clearChatHintIfEmpty() {
            var hint = el.messagesRoot.querySelector('.day-divider');
            if (hint && !el.messagesRoot.querySelector('.row')) {
                hint.remove();
            }
        }

        // Builds a chat row DOM node: bubble, photo, prediction meta and feedback buttons.
        function buildMessageRow(m) {
            var row = document.createElement('div');
            row.className = 'row ' + (m.role === 'user' ? 'user' : 'assistant');
            var bubble = document.createElement('div');
            bubble.className = 'bubble';

            if (m.image_data_url || m.image_url) {
                var img = document.createElement('img');
                img.className = 'attach-preview';
                img.alt = 'User photo';
                if (m.image_data_url) {
                    img.src = m.image_data_url;
                } else {
                    img.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="120" height="80"><rect fill="#ddd" width="100%" height="100%"/></svg>');
                    loadAuthedImage(img, m.image_url);
                }
                bubble.appendChild(img);
            }
            if (m.content && String(m.content).trim()) {
                var body = document.createElement('div');
                body.className = 'body';
                body.textContent = m.content;
                bubble.appendChild(body);
            }
            if (m.role === 'user' && m.class_prediction) {
                var meta = document.createElement('div');
                meta.className = 'meta-line';
                var pct = m.class_confidence > 0 ? Math.round(Number(m.class_confidence) * 100) : null;
                meta.textContent = formatPredictionName(m.class_prediction) + (pct != null ? ' · ' + pct + '%' : '');
                bubble.appendChild(meta);
            }

            if (m.role === 'assistant' && m.id) {
                var fb = document.createElement('div');
                fb.className = 'feedback-row';
                var rated = m.feedback_rating;
                [1, -1].forEach(function(r) {
                    var b = document.createElement('button');
                    b.type = 'button';
                    b.className = 'feedback-btn' + (rated === r ? ' active' : '');
                    b.setAttribute('data-rating', String(r));
                    b.setAttribute('data-feedback-for', String(m.id));
                    b.textContent = r === 1 ? '👍' : '👎';
                    b.disabled = rated != null;
                    b.addEventListener('click', function() { sendFeedback(m.id, r); });
                    fb.appendChild(b);
                });
                bubble.appendChild(fb);
            }

            row.appendChild(bubble);
            return row;
        }

        // Appends new messages to the chat without re-rendering existing ones.
        function appendMessages(messages) {
            if (!messages || !messages.length) return;
            clearChatHintIfEmpty();
            messages.forEach(function(m) {
                el.messagesRoot.appendChild(buildMessageRow(m));
            });
            updateOnboardingVisibility();
            scrollToBottom();
        }

        // Re-renders the whole chat, or shows a hint when there are no messages.
        function renderMessages(messages) {
            el.messagesRoot.innerHTML = '';
            if (!messages || !messages.length) {
                var hint = document.createElement('div');
                hint.className = 'day-divider';
                hint.textContent = 'Ask a question or attach a photo of an apple or leaf.';
                el.messagesRoot.appendChild(hint);
                updateOnboardingVisibility();
                return;
            }
            messages.forEach(function(m) {
                el.messagesRoot.appendChild(buildMessageRow(m));
            });
            updateOnboardingVisibility();
            scrollToBottom();
        }

        // Restores the stored session with history, or creates a new one.
        async function ensureSession() {
            var sid = sessionStorage.getItem(STORAGE_KEY);
            if (sid) {
                var hr = await apiFetch('/history?session_id=' + encodeURIComponent(sid), { method: 'GET' });
                if (hr.status === 401 && !isTelegramClient() && getStoredApiKey()) {
                    setStoredApiKey('');
                    sessionStorage.removeItem(STORAGE_KEY);
                    await ensureWebAuth();
                    sid = null;
                } else if (hr.status === 404) {
                    sessionStorage.removeItem(STORAGE_KEY);
                    sid = null;
                } else if (hr.ok) {
                    var hd = parseApiResponseJson(await hr.text());
                    sessionId = hd.session_id || sid;
                    if (hd.crop_id) {
                        cropId = hd.crop_id;
                        sessionStorage.setItem(CROP_STORAGE_KEY, cropId);
                        el.cropSelect.value = cropId;
                    }
                    renderMessages(hd.messages || []);
                    loadOnboarding(cropId);
                    return;
                } else {
                    sid = null;
                    sessionStorage.removeItem(STORAGE_KEY);
                }
            }
            var res = await apiFetch('/session', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json; charset=utf-8' },
                body: JSON.stringify({ crop_id: cropId })
            });
            var data = parseApiResponseJson(await res.text());
            if (!res.ok || !data.session_id) {
                throw new Error(data.error || 'Could not create session');
            }
            sessionId = data.session_id;
            if (data.crop_id) {
                cropId = data.crop_id;
                sessionStorage.setItem(CROP_STORAGE_KEY, cropId);
                el.cropSelect.value = cropId;
            }
            sessionStorage.setItem(STORAGE_KEY, sessionId);
            renderMessages([]);
            loadOnboarding(cropId);
        }

        // Sets or clears the attached photo and updates the attachment preview.
        function setPendingFile(file) {
            pendingFile = file || null;
            if (pendingObjectUrl) {
                URL.revokeObjectURL(pendingObjectUrl);
                pendingObjectUrl = null;
            }
            if (!file) {
                el.attachmentStrip.classList.remove('active');
                if (el.photoBetaNotice) el.photoBetaNotice.hidden = true;
                el.fileInput.value = '';
                return;
            }
            pendingObjectUrl = URL.createObjectURL(file);
            el.attachmentThumb.src = pendingObjectUrl;
            el.attachmentStrip.classList.add('active');
            if (el.photoBetaNotice && el.photoBetaNotice.textContent) el.photoBetaNotice.hidden = false;
        }

        // Toggles input controls and the typing indicator while a request is in flight.
        function setSending(on) {
            sending = on;
            el.sendBtn.disabled = on;
            el.attachBtn.disabled = on;
            el.inputText.disabled = on;
            el.typingLine.classList.toggle('active', on);
        }

        // Sends the current text/photo: multipart for photos, SSE stream for text.
        async function sendMessage() {
            if (sending) return;
            var text = (el.inputText.value || '').trim();
            if (!text && !pendingFile) {
                showToast('Enter text or attach a photo');
                return;
            }
            if (!sessionId) {
                try { await ensureSession(); } catch (e) {
                    showToast(e.message || 'Session error');
                    return;
                }
            }

            setSending(true);
            try {
                var res;
                if (pendingFile) {
                    var fd = new FormData();
                    fd.append('session_id', sessionId);
                    fd.append('crop_id', cropId);
                    fd.append('text', text);
                    fd.append('image', pendingFile, pendingFile.name || 'photo.jpg');
                    res = await apiFetch('/message', { method: 'POST', body: fd });
                } else {
                    res = await apiStreamFetch('/message/stream', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json; charset=utf-8' },
                        body: JSON.stringify({ session_id: sessionId, crop_id: cropId, text: text })
                    });
                    var streamCt = (res.headers.get('content-type') || '').toLowerCase();
                    if (streamCt.indexOf('text/event-stream') !== -1) {
                        await consumeMessageStream(res, text);
                        el.inputText.value = '';
                        setPendingFile(null);
                        autoResize();
                        return;
                    }
                    var data = parseApiResponseJson(await res.text());
                    if (data.session_id) {
                        sessionId = data.session_id;
                        sessionStorage.setItem(STORAGE_KEY, sessionId);
                    }
                    if (data.crop_id) {
                        cropId = data.crop_id;
                        sessionStorage.setItem(CROP_STORAGE_KEY, cropId);
                        el.cropSelect.value = cropId;
                    }
                    if (data.new_messages && data.new_messages.length) {
                        appendMessages(data.new_messages);
                    } else if (data.messages) {
                        renderMessages(data.messages);
                    }
                    if (!res.ok) {
                        showToast(data.error || ('Error ' + res.status));
                    } else if (data.error) {
                        showToast(data.error);
                    }
                    el.inputText.value = '';
                    setPendingFile(null);
                    autoResize();
                    return;
                }
                var data = parseApiResponseJson(await res.text());
                if (data.session_id) {
                    sessionId = data.session_id;
                    sessionStorage.setItem(STORAGE_KEY, sessionId);
                }
                if (data.crop_id) {
                    cropId = data.crop_id;
                    sessionStorage.setItem(CROP_STORAGE_KEY, cropId);
                    el.cropSelect.value = cropId;
                }
                if (data.new_messages && data.new_messages.length) {
                    appendMessages(data.new_messages);
                } else if (data.messages) {
                    renderMessages(data.messages);
                }
                if (!res.ok) {
                    showToast(data.error || ('Error ' + res.status));
                } else if (data.error) {
                    showToast(data.error);
                }
                el.inputText.value = '';
                setPendingFile(null);
                autoResize();
            } catch (e) {
                console.error(e);
                showToast(e.message || 'Network error');
            } finally {
                setSending(false);
            }
        }

        // Grows the input textarea with content, capped at 120px.
        function autoResize() {
            var ta = el.inputText;
            ta.style.height = 'auto';
            ta.style.height = Math.min(ta.scrollHeight, 120) + 'px';
        }

        el.attachBtn.addEventListener('click', function() { el.fileInput.click(); });
        el.fileInput.addEventListener('change', function() {
            var f = el.fileInput.files && el.fileInput.files[0];
            if (f) setPendingFile(f);
        });
        el.clearAttachment.addEventListener('click', function() { setPendingFile(null); });
        el.sendBtn.addEventListener('click', sendMessage);
        el.inputText.addEventListener('keydown', function(e) {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });
        el.inputText.addEventListener('input', autoResize);

        loadBranding().then(function() {
            return ensureWebAuth();
        }).then(function() {
            return loadCropsCatalog();
        }).then(function() {
            return ensureSession();
        }).then(function() {
            return loadOnboarding(cropId);
        }).catch(function(e) {
            console.error(e);
            showToast(e.message || 'Connection failed');
        });
