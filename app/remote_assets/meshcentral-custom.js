(function () {
    'use strict';

    var parametros = new URLSearchParams(window.location.search);
    var sessaoCoreTuner =
        parametros.get('coretuner') === '1' ||
        (window.args && String(window.args.coretuner) === '1');

    if (!sessaoCoreTuner || window.__coreTunerRemoteAutoStart) {
        return;
    }

    window.__coreTunerRemoteAutoStart = true;

    var INTERVALO_MS = 300;
    var TEMPO_ESTAVEL_MS = 1200;
    var LIMITE_CONEXAO_MS = 20000;
    var INTERVALO_REPETICAO_MS = 1800;
    var LIMITE_TOTAL_MS = 120000;
    var MAX_TENTATIVAS = 4;

    var inicio = Date.now();
    var paginaProntaDesde = 0;
    var conexaoIniciadaEm = 0;
    var proximaTentativaEm = 0;
    var tentativas = 0;
    var encerrado = false;

    function registrar(mensagem, erro) {
        var prefixo = '[CoreTuner Remote v6] ';
        if (erro) {
            console.error(prefixo + mensagem, erro);
        } else {
            console.log(prefixo + mensagem);
        }
    }

    function atualizarStatus(texto) {
        var elemento = document.getElementById('deskstatus');
        if (elemento && (!window.desktop || window.desktop.State !== 3)) {
            elemento.textContent = texto;
        }
    }

    function canalControlePronto() {
        return Boolean(window.meshserver && window.meshserver.State === 2);
    }

    function paginaPronta() {
        var computador = window.currentNode;

        return Boolean(
            document.readyState === 'complete' &&
            canalControlePronto() &&
            window.xxcurrentView === 11 &&
            computador &&
            computador._id &&
            computador.agent &&
            ((computador.conn & 1) !== 0) &&
            ((computador.agent.caps & 1) !== 0) &&
            typeof window.connectDesktop === 'function'
        );
    }

    function atualizarBotoesOficiais() {
        try {
            if (typeof window.updateDesktopButtons === 'function' && window.currentNode) {
                window.updateDesktopButtons();
            }
        } catch (erro) {
            registrar('Não foi possível atualizar os botões oficiais.', erro);
        }
    }

    function finalizar(mensagem, erro) {
        encerrado = true;
        window.clearInterval(temporizador);
        atualizarStatus(mensagem);
        registrar(mensagem, erro || null);
    }

    function limparTentativaPresa(agora) {
        registrar(
            'A conexão direta ficou parada no estado ' +
            (window.desktop ? window.desktop.State : 'desconhecido') +
            '; reiniciando.'
        );
        atualizarStatus('Reconectando automaticamente...');

        try {
            if (window.desktop && typeof window.desktop.Stop === 'function') {
                window.desktop.Stop();
            }
        } catch (erro) {
            registrar('Falha ao encerrar a tentativa anterior.', erro);
        }

        try {
            window.desktopNode = null;
            window.desktop = null;
        } catch (_) {
        }

        atualizarBotoesOficiais();
        conexaoIniciadaEm = 0;
        paginaProntaDesde = 0;
        proximaTentativaEm = agora + INTERVALO_REPETICAO_MS;
    }

    function iniciarConexaoDireta(agora) {
        tentativas += 1;
        conexaoIniciadaEm = agora;
        atualizarStatus('Conectando automaticamente...');
        registrar(
            'Iniciando tentativa direta ' + tentativas + ' de ' +
            MAX_TENTATIVAS + ' pelo MeshAgent.'
        );

        try {
            /*
             * O botão padrão desta versão chama connectDesktop(event, 3).
             * Em agentes Windows id 3/4, esse modo primeiro pede a lista de
             * sessões do Windows. Como o CoreTuner esconde essa seleção, a
             * sessão pode permanecer em "Desconectado" sem abrir meshrelay.
             * O modo 1 é o fluxo direto oficial do MeshAgent e abre o relay.
             */
            window.connectDesktop(null, 1);
        } catch (erro) {
            registrar('A conexão direta não pôde ser iniciada.', erro);
            conexaoIniciadaEm = 0;
            proximaTentativaEm = agora + INTERVALO_REPETICAO_MS;
        }
    }

    var temporizador = window.setInterval(function () {
        if (encerrado) {
            return;
        }

        var agora = Date.now();
        var conexao = window.desktop;

        if (conexao && conexao.State === 3) {
            finalizar('Conectado');
            return;
        }

        if (agora - inicio >= LIMITE_TOTAL_MS) {
            finalizar('Não foi possível conectar automaticamente.');
            return;
        }

        if (conexao) {
            if (
                conexaoIniciadaEm > 0 &&
                agora - conexaoIniciadaEm >= LIMITE_CONEXAO_MS
            ) {
                limparTentativaPresa(agora);
            }
            return;
        }

        if (tentativas >= MAX_TENTATIVAS) {
            finalizar('Não foi possível conectar automaticamente.');
            return;
        }

        if (agora < proximaTentativaEm) {
            return;
        }

        if (!paginaPronta()) {
            paginaProntaDesde = 0;
            return;
        }

        atualizarBotoesOficiais();

        if (!paginaProntaDesde) {
            paginaProntaDesde = agora;
            atualizarStatus('Preparando conexão segura...');
            registrar('Página, agente e canal de controle estão prontos.');
            return;
        }

        if (agora - paginaProntaDesde < TEMPO_ESTAVEL_MS) {
            return;
        }

        iniciarConexaoDireta(agora);
    }, INTERVALO_MS);
})();
