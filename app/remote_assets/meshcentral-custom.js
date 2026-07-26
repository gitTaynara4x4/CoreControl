(function () {
    'use strict';

    var parametrosIniciais = new URLSearchParams(window.location.search);
    var sessaoCoreTuner =
        parametrosIniciais.get('coretuner') === '1' ||
        (window.args && String(window.args.coretuner) === '1');

    if (!sessaoCoreTuner) {
        return;
    }

    var INTERVALO_MS = 500;
    var ESPERA_INICIAL_MS = 1500;
    var TEMPO_TENTATIVA_MS = 15000;
    var INTERVALO_NOVA_TENTATIVA_MS = 2500;
    var LIMITE_TOTAL_MS = 120000;
    var MAX_TENTATIVAS = 6;

    var inicio = Date.now();
    var tentativas = 0;
    var tentativaIniciadaEm = 0;
    var ultimaAcaoEm = 0;
    var encerrado = false;

    function registrar(mensagem, erro) {
        var prefixo = '[CoreTuner Remote] ';
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

    function paginaPronta() {
        var computador = window.currentNode;
        var botao = document.getElementById('connectbutton1');

        return (
            document.readyState === 'complete' &&
            window.xxcurrentView === 11 &&
            typeof window.connectDesktop === 'function' &&
            computador &&
            computador.agent &&
            ((computador.conn & 1) !== 0) &&
            ((computador.agent.caps & 1) !== 0) &&
            botao &&
            botao.disabled === false
        );
    }

    function limparConexaoParada() {
        if (!window.desktop) {
            return;
        }

        try {
            // Quando existe um objeto desktop, o mesmo método oficial faz a desconexão
            // e devolve a variável global desktop para null.
            window.connectDesktop(null, 0);
        } catch (erro) {
            registrar('Falha ao limpar a tentativa anterior.', erro);
            try {
                window.desktop.Stop();
            } catch (_) {
                // A próxima tentativa ainda poderá ocorrer depois do recarregamento.
            }
        }
    }

    function finalizar(mensagem, falha) {
        encerrado = true;
        clearInterval(temporizador);
        atualizarStatus(mensagem);
        registrar(mensagem, falha || null);
    }

    function iniciarTentativa() {
        tentativas += 1;
        tentativaIniciadaEm = Date.now();
        ultimaAcaoEm = tentativaIniciadaEm;
        atualizarStatus('Conectando automaticamente...');
        registrar('Iniciando tentativa ' + tentativas + ' de ' + MAX_TENTATIVAS + '.');

        // Pequeno atraso evita iniciar a captura no mesmo instante em que o
        // MeshCentral termina de selecionar o computador e montar o painel.
        setTimeout(function () {
            if (encerrado || window.desktop || !paginaPronta()) {
                return;
            }

            try {
                // É o mesmo fluxo do botão oficial "Conectar": o modo 3 permite
                // ao MeshCentral localizar a sessão ativa do Windows quando necessário.
                window.connectDesktop({ shiftKey: false }, 3);
            } catch (erro) {
                registrar('A chamada de conexão falhou.', erro);
                tentativaIniciadaEm = 0;
            }
        }, ESPERA_INICIAL_MS);
    }

    var temporizador = setInterval(function () {
        if (encerrado) {
            return;
        }

        var agora = Date.now();
        var conexao = window.desktop;

        if (conexao && conexao.State === 3) {
            finalizar('Conectado');
            return;
        }

        // Uma tentativa pode criar o objeto desktop, mas permanecer em State 0
        // (a tela mostra "Desconectar / Desconectado"). Nesse caso limpamos o
        // objeto e repetimos o fluxo oficial desde o início.
        if (
            conexao &&
            tentativaIniciadaEm > 0 &&
            conexao.State !== 3 &&
            agora - tentativaIniciadaEm >= TEMPO_TENTATIVA_MS
        ) {
            atualizarStatus('Reconectando automaticamente...');
            registrar('A tentativa ficou parada no estado ' + conexao.State + '; reiniciando.');
            limparConexaoParada();
            tentativaIniciadaEm = 0;
            ultimaAcaoEm = agora;
            return;
        }

        if (agora - inicio >= LIMITE_TOTAL_MS || tentativas >= MAX_TENTATIVAS) {
            finalizar('Falha na conexão automática. Use o botão Conectar.');
            return;
        }

        if (
            !conexao &&
            paginaPronta() &&
            agora - ultimaAcaoEm >= INTERVALO_NOVA_TENTATIVA_MS
        ) {
            iniciarTentativa();
        }
    }, INTERVALO_MS);
})();
