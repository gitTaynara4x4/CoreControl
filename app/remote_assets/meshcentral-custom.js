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
    var TEMPO_ESTAVEL_MS = 1500;
    var LIMITE_CONEXAO_MS = 20000;
    var INTERVALO_REPETICAO_MS = 2000;
    var LIMITE_TOTAL_MS = 120000;
    var MAX_TENTATIVAS = 4;

    var inicio = Date.now();
    var paginaProntaDesde = 0;
    var conexaoIniciadaEm = 0;
    var proximaTentativaEm = 0;
    var tentativas = 0;
    var encerrado = false;

    function registrar(mensagem, erro) {
        var prefixo = '[CoreTuner Remote v5] ';
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
        return Boolean(
            window.meshserver &&
            window.meshserver.State === 2
        );
    }

    function botaoConectar() {
        return document.getElementById('connectbutton1');
    }

    function botaoDesconectar() {
        return document.getElementById('disconnectbutton1');
    }

    function paginaPronta() {
        var computador = window.currentNode;
        var botao = botaoConectar();

        return Boolean(
            document.readyState === 'complete' &&
            canalControlePronto() &&
            window.xxcurrentView === 11 &&
            computador &&
            computador._id &&
            computador.agent &&
            ((computador.conn & 1) !== 0) &&
            ((computador.agent.caps & 1) !== 0) &&
            botao &&
            botao.disabled === false
        );
    }

    function finalizar(mensagem, erro) {
        encerrado = true;
        window.clearInterval(temporizador);
        atualizarStatus(mensagem);
        registrar(mensagem, erro || null);
    }

    function limparTentativaPresa(agora) {
        var botao = botaoDesconectar();

        registrar(
            'A conexão oficial ficou parada no estado ' +
            (window.desktop ? window.desktop.State : 'desconhecido') +
            '; reiniciando.'
        );
        atualizarStatus('Reconectando automaticamente...');

        try {
            // Usa o próprio botão oficial do MeshCentral. Isso preserva o fluxo
            // interno da versão instalada e evita manipular cookies de relay ou
            // chamar connectDesktop com parâmetros privados.
            if (botao && botao.disabled === false) {
                botao.click();
            } else if (window.desktop && typeof window.desktop.Stop === 'function') {
                window.desktop.Stop();
                window.desktopNode = null;
                window.desktop = null;
            }
        } catch (erro) {
            registrar('Falha ao encerrar a tentativa anterior.', erro);
            try {
                window.desktopNode = null;
                window.desktop = null;
            } catch (_) {
            }
        }

        conexaoIniciadaEm = 0;
        paginaProntaDesde = 0;
        proximaTentativaEm = agora + INTERVALO_REPETICAO_MS;
    }

    function iniciarPeloBotaoOficial(agora) {
        var botao = botaoConectar();
        if (!botao || botao.disabled) {
            return;
        }

        tentativas += 1;
        conexaoIniciadaEm = agora;
        atualizarStatus('Conectando automaticamente...');
        registrar(
            'Iniciando tentativa ' + tentativas + ' de ' +
            MAX_TENTATIVAS + ' pelo botão oficial.'
        );

        try {
            // O onclick do botão é mantido pelo próprio MeshCentral e chama o
            // modo correto para o Mesh Agent da versão em execução.
            botao.click();
        } catch (erro) {
            registrar('O botão oficial não iniciou a conexão.', erro);
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

        if (!paginaProntaDesde) {
            paginaProntaDesde = agora;
            atualizarStatus('Preparando conexão segura...');
            return;
        }

        if (agora - paginaProntaDesde < TEMPO_ESTAVEL_MS) {
            return;
        }

        iniciarPeloBotaoOficial(agora);
    }, INTERVALO_MS);
})();
