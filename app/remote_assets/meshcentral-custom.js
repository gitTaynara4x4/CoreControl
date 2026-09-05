(function () {
    'use strict';

    var VERSAO = 'CoreControl Remote v10.10-quick-controls';
    var params = new URLSearchParams(window.location.search || '');

    function log(msg, erro) {
        if (erro) console.error('[' + VERSAO + '] ' + msg, erro);
        else console.log('[' + VERSAO + '] ' + msg);
    }

    function param(nome) {
        var valor = params.get(nome);
        if (valor) return String(valor);
        try {
            if (window.args && window.args[nome] != null) return String(window.args[nome]);
        } catch (_) {}
        return '';
    }

    function base64UrlDecodeUtf8(valor) {
        try {
            var normal = String(valor || '').replace(/-/g, '+').replace(/_/g, '/');
            while (normal.length % 4) normal += '=';
            var bin = window.atob(normal);
            var bytes = new Uint8Array(bin.length);
            for (var i = 0; i < bin.length; i += 1) bytes[i] = bin.charCodeAt(i);
            return new TextDecoder('utf-8').decode(bytes);
        } catch (_) {
            return '';
        }
    }

    function extrairSessao() {
        var marker = param('coretuner');
        if (!marker) return { ativa: false, node: '' };
        if (marker.indexOf('1_') === 0 && marker.length > 2) {
            return { ativa: true, node: base64UrlDecodeUtf8(marker.slice(2)) };
        }
        if (marker === '1') {
            return { ativa: true, node: param('ctnode') || param('gotonode') || '' };
        }
        return { ativa: false, node: '' };
    }

    var sessao = extrairSessao();
    if (!sessao.ativa || window.__coreTunerRemoteAutoStart) return;

    window.__coreTunerRemoteAutoStart = true;
    window.__coreControlRemoteVersion = VERSAO;

    var nodeAlvo = String(sessao.node || '').trim();
    try {
        if (nodeAlvo) window.sessionStorage.setItem('coretuner.remote.node', nodeAlvo);
        else nodeAlvo = window.sessionStorage.getItem('coretuner.remote.node') || '';
    } catch (_) {}

    function idCurto(id) {
        var texto = String(id || '').trim();
        if (!texto) return '';
        var partes = texto.split('/');
        return partes[partes.length - 1];
    }

    function listarNodes() {
        var origem = window.nodes;
        if (Array.isArray(origem)) return origem.filter(function (n) { return n && n._id; });
        if (origem && typeof origem === 'object') {
            return Object.keys(origem).map(function (k) { return origem[k]; }).filter(function (n) { return n && n._id; });
        }
        return [];
    }

    function localizarNodeExato() {
        var alvo = idCurto(nodeAlvo);
        if (!alvo) return null;
        var lista = listarNodes();
        for (var i = 0; i < lista.length; i += 1) {
            if (String(lista[i]._id) === String(nodeAlvo) || idCurto(lista[i]._id) === alvo) return lista[i];
        }
        return null;
    }

    function garantirNode() {
        var n = localizarNodeExato();
        if (!n) return null;
        window.currentNode = n;
        window.desktopNode = n;
        return n;
    }

    function setDeskControl(valor) {
        var box = document.getElementById('DeskControl');
        if (box) box.checked = !!valor;
        try { if (typeof window.putstore === 'function') window.putstore('DeskControl', valor ? 1 : 0); } catch (_) {}
    }

    function toggleSidePanel(forceHide) {
        var proc = document.getElementById('p10rightOfButtons') || document.getElementById('deskarea3x') || document.querySelector('td[style*="width:320px"]');
        if (!proc) return;
        var esconder = typeof forceHide === 'boolean' ? forceHide : proc.style.display !== 'none';
        proc.style.display = esconder ? 'none' : '';
        document.body.classList.toggle('cc-remote-side-hidden', esconder);
    }

    function syncQuickBar() {
        var bar = document.getElementById('ccQuickBar');
        if (!bar) return;
        var node = window.desktopNode || window.currentNode || localizarNodeExato();
        var state = document.getElementById('deskstatus');
        var label = document.getElementById('ccQuickBarStatus');
        if (label) {
            var conectado = window.desktop && window.desktop.State === 3;
            label.textContent = conectado ? 'Conectado' : ((state && state.textContent) || (node ? 'Pronto para conectar' : 'Aguardando PC'));
        }
        var connectBtn = document.getElementById('connectbutton1');
        if (connectBtn && node && node.agent && ((node.conn & 1) !== 0)) {
            connectBtn.disabled = false;
            connectBtn.removeAttribute('disabled');
        }
        var inputToggle = document.getElementById('ccQuickBarInput');
        var deskControl = document.getElementById('DeskControl');
        if (inputToggle && deskControl) inputToggle.checked = !!deskControl.checked;
    }

    function ensureQuickBar() {
        if (document.getElementById('ccQuickBar')) return;
        var style = document.createElement('style');
        style.textContent = '' +
            '#ccQuickBar{position:fixed;top:10px;right:86px;z-index:99999;display:flex;align-items:center;gap:8px;flex-wrap:wrap;padding:8px 10px;border-radius:12px;background:rgba(8,26,56,.96);color:#fff;box-shadow:0 12px 28px rgba(8,18,36,.35);font-family:Inter,Arial,sans-serif}' +
            '#ccQuickBar strong{font-size:12px;line-height:1;margin-right:2px}' +
            '#ccQuickBar small{font-size:10px;line-height:1.2;opacity:.78}' +
            '#ccQuickBar .cc-btn{height:30px;padding:0 10px;border:1px solid rgba(255,255,255,.16);border-radius:9px;background:#102545;color:#fff;cursor:pointer}' +
            '#ccQuickBar .cc-btn:hover{background:#17335f}' +
            '#ccQuickBar label{display:flex;align-items:center;gap:6px;font-size:11px;padding:0 8px;height:30px;border:1px solid rgba(255,255,255,.16);border-radius:9px;background:#102545}' +
            '#ccQuickBar input[type="checkbox"]{margin:0}' +
            'body.cc-remote-side-hidden #deskarea3x, body.cc-remote-side-hidden #p10rightOfButtons{display:none !important}';
        document.head.appendChild(style);

        var bar = document.createElement('div');
        bar.id = 'ccQuickBar';
        bar.innerHTML = '' +
            '<strong>CoreControl</strong>' +
            '<small id="ccQuickBarStatus">Preparando...</small>' +
            '<button type="button" class="cc-btn" id="ccQuickBarConnect">Conectar</button>' +
            '<button type="button" class="cc-btn" id="ccQuickBarPanel">Maximizar</button>' +
            '<label><input type="checkbox" id="ccQuickBarInput"> Entrada</label>';
        document.body.appendChild(bar);

        document.getElementById('ccQuickBarConnect').onclick = function () {
            var n = garantirNode();
            if (!n) return;
            setDeskControl(true);
            try { if (typeof window.connectDesktop === 'function') window.connectDesktop(null, 3); } catch (erro) { log('Falha ao conectar pelo atalho.', erro); }
            window.setTimeout(syncQuickBar, 250);
        };
        document.getElementById('ccQuickBarPanel').onclick = function () {
            var esconder = !document.body.classList.contains('cc-remote-side-hidden');
            toggleSidePanel(esconder);
            this.textContent = esconder ? 'Minimizar' : 'Maximizar';
        };
        document.getElementById('ccQuickBarInput').onchange = function () { setDeskControl(this.checked); };
    }

    function tentativaDireta() {
        var n = garantirNode();
        if (!n || !n.agent || ((n.conn & 1) === 0)) return false;
        var connectBtn = document.getElementById('connectbutton1');
        if (connectBtn) {
            connectBtn.disabled = false;
            connectBtn.removeAttribute('disabled');
        }
        setDeskControl(true);
        if (window.desktop && window.desktop.State === 3) return true;
        try {
            if (typeof window.connectDesktop === 'function') {
                window.connectDesktop(null, 3);
                log('Conectando via bind direto do front.');
                return true;
            }
        } catch (erro) {
            log('Falha no bind direto.', erro);
        }
        return false;
    }

    if (!nodeAlvo) {
        log('Sessão CoreControl sem node alvo.');
        return;
    }

    ensureQuickBar();
    syncQuickBar();

    var inicio = Date.now();
    var LIMITE_TOTAL_MS = 90000;
    var timer = window.setInterval(function () {
        ensureQuickBar();
        syncQuickBar();
        if (window.desktop && window.desktop.State === 3) {
            syncQuickBar();
            return;
        }
        if (Date.now() - inicio > LIMITE_TOTAL_MS) {
            log('Tempo limite da conexão automática atingido.');
            window.clearInterval(timer);
            return;
        }
        tentativaDireta();
    }, 500);
})();
