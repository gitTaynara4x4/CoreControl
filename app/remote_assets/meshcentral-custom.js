(function () {
    'use strict';

    // CoreControl Remote v10.10
    // Fluxo intencionalmente simples: o CoreControl já abre a página Desktop.
    // Não use gotoDevice/gotoNode aqui. Apenas associe o node autorizado que já
    // está em window.nodes ao desktopNode e conecte usando o mesmo modo do botão
    // oficial do MeshCentral: connectDesktop(event, 3).
    var VERSAO = 'CoreControl Remote v10.10-front-direct-bind-fit';
    var STORAGE_NODE = 'coretuner.remote.node';
    var STORAGE_TS = 'coretuner.remote.ts';
    var MAX_SESSION_AGE_MS = 10 * 60 * 1000;
    var MAX_WAIT_MS = 90000;
    var POLL_MS = 250;
    var RETRY_CONNECT_MS = 6000;

    window.__coreControlRemoteVersion = VERSAO;

    function log(msg, extra) {
        if (extra !== undefined) console.log('[' + VERSAO + '] ' + msg, extra);
        else console.log('[' + VERSAO + '] ' + msg);
    }

    function base64UrlDecodeUtf8(valor) {
        try {
            var normal = String(valor || '').replace(/-/g, '+').replace(/_/g, '/');
            while (normal.length % 4) normal += '=';
            var bin = window.atob(normal);
            var bytes = new Uint8Array(bin.length);
            for (var i = 0; i < bin.length; i += 1) bytes[i] = bin.charCodeAt(i);
            if (window.TextDecoder) return new TextDecoder('utf-8').decode(bytes);
            var escaped = '';
            for (var j = 0; j < bytes.length; j += 1) escaped += '%' + ('0' + bytes[j].toString(16)).slice(-2);
            return decodeURIComponent(escaped);
        } catch (_) {
            return '';
        }
    }

    function idCurto(id) {
        var partes = String(id || '').trim().split('/');
        return partes[partes.length - 1] || '';
    }

    function alvoDaUrl() {
        try {
            var p = new URLSearchParams(window.location.search || '');
            var marker = p.get('coretuner') || '';
            if (marker.indexOf('1_') === 0 && marker.length > 2) {
                return base64UrlDecodeUtf8(marker.slice(2));
            }
            return p.get('ctnode') || p.get('gotonode') || '';
        } catch (_) {
            return '';
        }
    }

    function alvoSalvo() {
        try {
            var node = window.sessionStorage.getItem(STORAGE_NODE) || '';
            var ts = Number(window.sessionStorage.getItem(STORAGE_TS) || '0');
            // Compatibilidade: versões anteriores salvaram apenas o node.
            if (!ts && node) return node;
            if (node && ts && (Date.now() - ts) <= MAX_SESSION_AGE_MS) return node;
        } catch (_) {}
        return '';
    }

    function salvarAlvo(node) {
        try {
            if (!node) return;
            window.sessionStorage.setItem(STORAGE_NODE, String(node));
            window.sessionStorage.setItem(STORAGE_TS, String(Date.now()));
        } catch (_) {}
    }

    var nodeAlvo = String(alvoDaUrl() || alvoSalvo() || '').trim();
    if (!nodeAlvo) {
        window.__coreControlRemoteDebug = { ativa: false, versao: VERSAO, motivo: 'sem-node-alvo' };
        return;
    }

    salvarAlvo(nodeAlvo);

    if (window.__coreControlDirectBindStarted) return;
    window.__coreControlDirectBindStarted = true;
    window.__coreTunerRemoteAutoStart = true;

    var inicio = Date.now();
    var ultimaTentativa = 0;
    var nodeVinculado = null;

    window.__coreControlRemoteDebug = {
        ativa: true,
        versao: VERSAO,
        nodeAlvo: nodeAlvo,
        fase: 'aguardando-nodes'
    };

    function listarNodes() {
        try {
            if (Array.isArray(window.nodes)) return window.nodes.filter(function (n) { return n && n._id; });
            if (window.nodes && typeof window.nodes === 'object') {
                return Object.keys(window.nodes).map(function (k) { return window.nodes[k]; })
                    .filter(function (n) { return n && n._id; });
            }
        } catch (_) {}
        return [];
    }

    function localizarNode() {
        var alvoCurto = idCurto(nodeAlvo);
        var lista = listarNodes();
        for (var i = 0; i < lista.length; i += 1) {
            var n = lista[i];
            if (String(n._id) === nodeAlvo || idCurto(n._id) === alvoCurto) return n;
        }
        return null;
    }

    function status(texto) {
        try {
            var el = document.getElementById('deskstatus');
            if (el) el.textContent = texto;
        } catch (_) {}
    }

    function aplicarTelaInteira() {
        try {
            var css = [
                'html,body{width:100%!important;height:100%!important;margin:0!important;overflow:hidden!important;background:#777!important;}',
                '#p10,#p10desktop,#deskarea0,#deskarea1,#deskarea2,#DeskParent{max-width:100vw!important;max-height:100vh!important;overflow:hidden!important;}',
                '#DeskParent{width:100vw!important;height:100vh!important;display:flex!important;align-items:center!important;justify-content:center!important;background:#777!important;}',
                'canvas#Desk,#Desk{max-width:100%!important;max-height:100%!important;width:auto!important;height:auto!important;object-fit:contain!important;}'
            ].join('');
            var style = document.getElementById('corecontrol-fit-to-window');
            if (!style) {
                style = document.createElement('style');
                style.id = 'corecontrol-fit-to-window';
                (document.head || document.documentElement).appendChild(style);
            }
            if (style.textContent !== css) style.textContent = css;
        } catch (_) {}
    }

    function vincularNode(n) {
        if (!n) return false;

        // Esta é a parte que resolveu o caso real da Luiza no Console.
        window.currentNode = n;
        window.desktopNode = n;
        nodeVinculado = n;

        try {
            var conectar = document.getElementById('connectbutton1');
            if (conectar) {
                conectar.disabled = false;
                conectar.removeAttribute('disabled');
            }
        } catch (_) {}

        window.__coreControlRemoteDebug.fase = 'node-vinculado';
        window.__coreControlRemoteDebug.node = n._id;
        window.__coreControlRemoteDebug.nome = n.name || '';
        window.__coreControlRemoteDebug.conn = n.conn;
        return true;
    }

    function habilitarEntrada() {
        try {
            var entrada = document.getElementById('DeskControl');
            if (entrada) {
                entrada.checked = true;
                entrada.removeAttribute('disabled');
            }

            if (typeof window.putstore === 'function') {
                window.putstore('DeskControl', 1);
            }

            // Em algumas builds o checkbox existe mas só é atualizado depois
            // que os controles do desktop são redesenhados. Não chamamos
            // updateDesktopButtons antes da conexão porque ele pode zerar o node.
            return Boolean(entrada);
        } catch (_) {
            return false;
        }
    }

    function estaConectado() {
        try {
            return Boolean(window.desktop && window.desktop.State === 3);
        } catch (_) {
            return false;
        }
    }

    function conectar(n) {
        if (!n || typeof window.connectDesktop !== 'function') return false;
        if (!n.agent || ((Number(n.conn || 0) & 1) === 0)) return false;
        if (!document.getElementById('connectbutton1')) return false;
        if (estaConectado()) return true;
        if ((Date.now() - ultimaTentativa) < RETRY_CONNECT_MS) return false;

        ultimaTentativa = Date.now();

        // Reaplica imediatamente antes da chamada. Rotinas internas do
        // MeshCentral podem limpar desktopNode enquanto a página inicializa.
        vincularNode(n);
        habilitarEntrada();

        try {
            status('Conectando...');
            window.__coreControlRemoteDebug.fase = 'connectDesktop';
            window.connectDesktop(null, 3);
            log('Node vinculado diretamente e connectDesktop(null, 3) executado.', {
                nome: n.name,
                id: n._id,
                conn: n.conn
            });
            return true;
        } catch (erro) {
            window.__coreControlRemoteDebug.fase = 'erro-connectDesktop';
            window.__coreControlRemoteDebug.erro = String(erro && erro.message ? erro.message : erro);
            console.error('[' + VERSAO + '] connectDesktop falhou.', erro);
            return false;
        }
    }

    log('Sessão CoreControl detectada. Aguardando node autorizado: ' + nodeAlvo);

    var timer = window.setInterval(function () {
        if ((Date.now() - inicio) > MAX_WAIT_MS) {
            window.clearInterval(timer);
            window.__coreControlRemoteDebug.fase = 'timeout';
            status('Não foi possível conectar automaticamente.');
            log('Tempo limite aguardando conexão remota.');
            return;
        }

        var n = localizarNode();
        if (!n) {
            window.__coreControlRemoteDebug.fase = 'aguardando-node';
            window.__coreControlRemoteDebug.nodes = listarNodes().length;
            return;
        }

        // Sempre mantenha o node correto associado. Isto é deliberado e
        // substitui o antigo fluxo com gotoDevice(), que causava o estado
        // "Desconectado" + botão Conectar desabilitado.
        vincularNode(n);

        if (estaConectado()) {
            habilitarEntrada();
            status('Conectado');
            aplicarTelaInteira();
            window.__coreControlRemoteDebug.fase = 'conectado';
            window.__coreControlRemoteDebug.input = true;
            window.clearInterval(timer);
            log('Desktop conectado; mouse e teclado habilitados.');
            return;
        }

        conectar(n);
        aplicarTelaInteira();
    }, POLL_MS);

    aplicarTelaInteira();
    window.addEventListener('resize', aplicarTelaInteira);
})();
