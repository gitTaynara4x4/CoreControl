(function () {
    'use strict';

    var parametros = new URLSearchParams(window.location.search);

    function obterParametro(nome) {
        var valor = parametros.get(nome);
        if (valor) {
            return String(valor);
        }

        try {
            if (window.args && window.args[nome] != null) {
                return String(window.args[nome]);
            }
        } catch (_) {
        }

        return '';
    }

    var sessaoCoreTuner =
        obterParametro('coretuner') === '1';

    if (!sessaoCoreTuner || window.__coreTunerRemoteAutoStart) {
        return;
    }

    window.__coreTunerRemoteAutoStart = true;

    /*
     * O MeshCentral consome e remove "gotonode" depois do login por token.
     * O CoreControl envia também "ctnode", que é um parâmetro próprio e fica
     * preservado no redirecionamento. O sessionStorage é uma proteção extra
     * caso outra versão do MeshCentral também remova esse parâmetro da URL.
     */
    var nodeAlvo =
        obterParametro('ctnode') ||
        obterParametro('gotonode');

    try {
        if (nodeAlvo) {
            window.sessionStorage.setItem('coretuner.remote.node', nodeAlvo);
        } else {
            nodeAlvo = window.sessionStorage.getItem('coretuner.remote.node') || '';
        }
    } catch (_) {
    }

    var INTERVALO_MS = 300;
    var TEMPO_ESTAVEL_MS = 1000;
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
    var dispositivoAberto = false;
    var dispositivoSelecionado = null;

    function registrar(mensagem, erro) {
        var prefixo = '[CoreControl Remote v7] ';
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

    function idCurto(valor) {
        var texto = String(valor || '').trim();
        if (!texto) {
            return '';
        }
        var partes = texto.split('/');
        return partes[partes.length - 1];
    }

    function listarComputadores() {
        var origem = window.nodes;
        var lista = [];
        var chave;

        if (Array.isArray(origem)) {
            return origem.filter(function (item) {
                return Boolean(item && item._id);
            });
        }

        if (origem && typeof origem === 'object') {
            for (chave in origem) {
                if (
                    Object.prototype.hasOwnProperty.call(origem, chave) &&
                    origem[chave] &&
                    origem[chave]._id
                ) {
                    lista.push(origem[chave]);
                }
            }
        }

        return lista;
    }

    function localizarComputador() {
        var lista = listarComputadores();
        var alvoCurto = idCurto(nodeAlvo);
        var atual = window.currentNode;
        var i;

        if (
            atual &&
            atual._id &&
            (!alvoCurto || idCurto(atual._id) === alvoCurto)
        ) {
            return atual;
        }

        if (alvoCurto) {
            for (i = 0; i < lista.length; i += 1) {
                if (
                    String(lista[i]._id) === String(nodeAlvo) ||
                    idCurto(lista[i]._id) === alvoCurto
                ) {
                    return lista[i];
                }
            }
        }

        /*
         * Compatibilidade com links antigos sem ctnode. Só usamos fallback
         * quando existe exatamente um computador visível, para nunca abrir
         * o equipamento errado em contas com vários computadores.
         */
        if (!alvoCurto && lista.length === 1) {
            registrar(
                'O link não trouxe ctnode; usando o único computador disponível: ' +
                lista[0]._id
            );
            return lista[0];
        }

        return null;
    }

    function selecionarComputadorCorreto() {
        var computador = localizarComputador();

        if (!computador) {
            return null;
        }

        dispositivoSelecionado = computador;

        /*
         * Esta é a correção central: a tela do MeshCentral pode carregar nodes
         * e manter currentNode vazio após o login temporário. Todos os botões
         * e o desktop dependem de currentNode.agent/currentNode._id.
         */
        window.currentNode = computador;

        if (!dispositivoAberto && typeof window.gotoDevice === 'function') {
            try {
                dispositivoAberto = true;
                window.gotoDevice(computador._id, 11);

                /*
                 * Algumas versões limpam currentNode durante gotoDevice.
                 * Reaplicamos o mesmo objeto validado logo em seguida.
                 */
                window.currentNode = computador;
                registrar('Computador correto selecionado: ' + computador._id);
            } catch (erro) {
                dispositivoAberto = false;
                registrar('Não foi possível abrir o computador correto.', erro);
                return null;
            }
        }

        return computador;
    }

    function canalControlePronto() {
        return Boolean(window.meshserver && window.meshserver.State === 2);
    }

    function paginaPronta() {
        var computador = selecionarComputadorCorreto();

        return Boolean(
            document.readyState === 'complete' &&
            canalControlePronto() &&
            window.xxcurrentView === 11 &&
            computador &&
            window.currentNode &&
            window.currentNode._id === computador._id &&
            computador._id &&
            computador.agent &&
            ((computador.conn & 1) !== 0) &&
            ((computador.agent.caps & 1) !== 0) &&
            typeof window.connectDesktop === 'function'
        );
    }

    function atualizarBotoesOficiais() {
        try {
            if (
                typeof window.updateDesktopButtons === 'function' &&
                window.currentNode
            ) {
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
            'A conexão ficou parada no estado ' +
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

        /* Mantém o computador correto selecionado durante a nova tentativa. */
        if (dispositivoSelecionado) {
            window.currentNode = dispositivoSelecionado;
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
            'Iniciando tentativa ' + tentativas + ' de ' +
            MAX_TENTATIVAS + ' pelo MeshAgent.'
        );

        try {
            window.currentNode = dispositivoSelecionado || localizarComputador();
            atualizarBotoesOficiais();
            window.connectDesktop(null, 1);
        } catch (erro) {
            registrar('A conexão não pôde ser iniciada.', erro);
            conexaoIniciadaEm = 0;
            proximaTentativaEm = agora + INTERVALO_REPETICAO_MS;
        }
    }

    var temporizador = window.setInterval(function () {
        var agora;
        var conexao;

        if (encerrado) {
            return;
        }

        agora = Date.now();
        conexao = window.desktop;

        if (conexao && conexao.State === 3) {
            finalizar('Conectado');
            return;
        }

        if (agora - inicio >= LIMITE_TOTAL_MS) {
            if (!dispositivoSelecionado) {
                finalizar(
                    nodeAlvo
                        ? 'O computador solicitado não foi encontrado no MeshCentral.'
                        : 'O link de acesso não informou qual computador deve ser aberto.'
                );
            } else {
                finalizar('Não foi possível conectar automaticamente.');
            }
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
            registrar('Computador, página, agente e canal de controle estão prontos.');
            return;
        }

        if (agora - paginaProntaDesde < TEMPO_ESTAVEL_MS) {
            return;
        }

        iniciarConexaoDireta(agora);
    }, INTERVALO_MS);
})();
