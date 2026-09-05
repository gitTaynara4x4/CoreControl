(function () {
    'use strict';

    var VERSAO = 'CoreControl Remote v10.6-control';
    var params = new URLSearchParams(window.location.search || '');

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

        // Formato novo: coretuner=1_<node-id em base64url>
        if (marker.indexOf('1_') === 0 && marker.length > 2) {
            return { ativa: true, node: base64UrlDecodeUtf8(marker.slice(2)) };
        }

        // Compatibilidade com links antigos.
        if (marker === '1') {
            return {
                ativa: true,
                node: param('ctnode') || param('gotonode') || ''
            };
        }

        return { ativa: false, node: '' };
    }

    var sessao = extrairSessao();
    if (!sessao.ativa || window.__coreTunerRemoteAutoStart) return;

    window.__coreTunerRemoteAutoStart = true;
    window.__coreControlRemoteVersion = VERSAO;

    var nodeAlvo = String(sessao.node || '').trim();

    try {
        if (nodeAlvo) {
            window.sessionStorage.setItem('coretuner.remote.node', nodeAlvo);
        } else {
            nodeAlvo = window.sessionStorage.getItem('coretuner.remote.node') || '';
        }
    } catch (_) {}

    var inicio = Date.now();
    var LIMITE_TOTAL_MS = 90000;
    var INTERVALO_MS = 300;
    var ESTAVEL_MS = 900;
    var paginaProntaDesde = 0;
    var dispositivoSelecionado = null;
    var dispositivoAberto = false;
    var conexaoIniciada = false;

    function log(msg, erro) {
        if (erro) console.error('[' + VERSAO + '] ' + msg, erro);
        else console.log('[' + VERSAO + '] ' + msg);
    }

    function status(texto) {
        var el = document.getElementById('deskstatus');
        if (el && (!window.desktop || window.desktop.State !== 3)) el.textContent = texto;
    }

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
            return Object.keys(origem).map(function (k) { return origem[k]; })
                .filter(function (n) { return n && n._id; });
        }
        return [];
    }

    function localizarNodeExato() {
        var alvo = idCurto(nodeAlvo);
        if (!alvo) return null;
        var lista = listarNodes();
        for (var i = 0; i < lista.length; i += 1) {
            if (String(lista[i]._id) === String(nodeAlvo) || idCurto(lista[i]._id) === alvo) {
                return lista[i];
            }
        }
        return null;
    }

    function selecionarNode() {
        var n = localizarNodeExato();
        if (!n) return null;
        dispositivoSelecionado = n;
        window.currentNode = n;

        if (!dispositivoAberto && typeof window.gotoDevice === 'function') {
            try {
                dispositivoAberto = true;
                window.gotoDevice(n._id, 11);
                window.currentNode = n;
                log('Computador exato selecionado: ' + n._id);
            } catch (erro) {
                dispositivoAberto = false;
                log('Falha ao abrir o computador selecionado.', erro);
                return null;
            }
        }
        return n;
    }

    function prontoParaConectar(n) {
        return Boolean(
            document.readyState === 'complete' &&
            window.meshserver && window.meshserver.State === 2 &&
            window.xxcurrentView === 11 &&
            n && n._id && n.agent &&
            ((n.conn & 1) !== 0) &&
            ((n.agent.caps & 1) !== 0) &&
            typeof window.connectDesktop === 'function'
        );
    }

    function habilitarControle() {
        // O MeshCentral persiste o checkbox "Input" no navegador. Se uma
        // sessão anterior foi aberta em modo visualização, o valor 0 pode ser
        // reutilizado nas sessões seguintes mesmo quando o usuário possui
        // RemoteControl. Para sessões autorizadas pelo CoreControl, force o
        // controle de mouse/teclado. As permissões continuam sendo validadas
        // pelo próprio MeshCentral/Agent no servidor.
        try {
            var input = document.getElementById('DeskControl');
            if (!input) return false;
            input.checked = true;
            if (typeof window.putstore === 'function') {
                window.putstore('DeskControl', 1);
            } else {
                window.localStorage.setItem('DeskControl', '1');
            }
            var span = document.getElementById('DeskControlSpan');
            if (span) span.style.color = '';
            return true;
        } catch (erro) {
            log('Não foi possível habilitar o controle de entrada.', erro);
            return false;
        }
    }

    function iniciarDesktop(n) {
        if (conexaoIniciada) return;
        conexaoIniciada = true;
        window.currentNode = n;
        status('Conectando automaticamente...');
        try {
            habilitarControle();
            if (typeof window.updateDesktopButtons === 'function') window.updateDesktopButtons();
            // updateDesktopButtons pode restaurar o valor persistido, então
            // reaplicamos antes de criar a sessão KVM.
            habilitarControle();
            window.connectDesktop(null, 1);
            log('connectDesktop iniciado com mouse e teclado habilitados.');
        } catch (erro) {
            conexaoIniciada = false;
            log('Falha ao iniciar connectDesktop.', erro);
        }
    }

    if (!nodeAlvo) {
        status('O link remoto não informou o computador.');
        log('Sessão CoreControl sem node alvo.');
        return;
    }

    log('Automação iniciada. Nó transportado no marcador coretuner.');

    var timer = window.setInterval(function () {
        if (window.desktop && window.desktop.State === 3) {
            habilitarControle();
            status('Conectado — controle de mouse e teclado ativo');
            log('Desktop conectado com Input habilitado.');
            window.clearInterval(timer);
            return;
        }

        if (Date.now() - inicio > LIMITE_TOTAL_MS) {
            status(dispositivoSelecionado ? 'Não foi possível conectar automaticamente.' : 'Computador remoto não encontrado.');
            log('Tempo limite da conexão automática atingido.');
            window.clearInterval(timer);
            return;
        }

        var n = selecionarNode();
        if (!n) {
            paginaProntaDesde = 0;
            return;
        }

        // Reaplica porque algumas rotinas internas do MeshCentral zeram currentNode.
        window.currentNode = n;

        if (!prontoParaConectar(n)) {
            paginaProntaDesde = 0;
            return;
        }

        if (!paginaProntaDesde) {
            paginaProntaDesde = Date.now();
            return;
        }

        if (Date.now() - paginaProntaDesde >= ESTAVEL_MS) iniciarDesktop(n);
    }, INTERVALO_MS);
})();
