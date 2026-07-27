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

    var INTERVALO_MS = 250;
    var ESTABILIDADE_MS = 2500;
    var RESPOSTA_COOKIE_MS = 5000;
    var RESPOSTA_SESSAO_MS = 12000;
    var CONEXAO_MS = 25000;
    var INTERVALO_REPETICAO_MS = 3000;
    var LIMITE_TOTAL_MS = 150000;
    var MAX_TENTATIVAS = 5;

    var inicio = Date.now();
    var prontoDesde = 0;
    var proximaAcaoEm = 0;
    var cookieSolicitadoEm = 0;
    var cookieAnterior = '';
    var cookieAtualizado = false;
    var tentativaIniciadaEm = 0;
    var tentativas = 0;
    var encerrado = false;

    function registrar(mensagem, erro) {
        var prefixo = '[CoreTuner Remote v4] ';
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
            window.meshserver.State === 2 &&
            typeof window.meshserver.send === 'function'
        );
    }

    function paginaPronta() {
        var computador = window.currentNode;
        var botao = document.getElementById('connectbutton1');
        var corpoVisivel = document.body &&
            window.getComputedStyle(document.body).display !== 'none';

        return Boolean(
            document.readyState === 'complete' &&
            corpoVisivel &&
            canalControlePronto() &&
            window.xxcurrentView === 11 &&
            typeof window.connectDesktop === 'function' &&
            computador &&
            computador._id &&
            computador.agent &&
            ((computador.conn & 1) !== 0) &&
            ((computador.agent.caps & 1) !== 0) &&
            botao &&
            botao.disabled === false
        );
    }

    function cookiesRelayPresentes() {
        return Boolean(
            typeof window.authCookie === 'string' &&
            window.authCookie.length > 20 &&
            typeof window.authRelayCookie === 'string' &&
            window.authRelayCookie.length > 20
        );
    }

    function solicitarCookieAtualizado() {
        if (!canalControlePronto() || cookieSolicitadoEm > 0) {
            return;
        }

        cookieAnterior = String(window.authRelayCookie || '');
        cookieSolicitadoEm = Date.now();
        cookieAtualizado = false;
        atualizarStatus('Preparando canal seguro...');
        registrar('Solicitando cookie de relay vinculado à sessão de controle atual.');

        try {
            window.meshserver.send({ action: 'authcookie' });
        } catch (erro) {
            registrar('Falha ao solicitar o cookie de relay.', erro);
            cookieSolicitadoEm = 0;
        }
    }

    function verificarCookieAtualizado(agora) {
        if (!cookiesRelayPresentes()) {
            return false;
        }

        if (!cookieSolicitadoEm) {
            return false;
        }

        if (String(window.authRelayCookie) !== cookieAnterior) {
            cookieAtualizado = true;
        }

        // O cookie é criptografado e normalmente muda a cada resposta. Caso a
        // implementação devolva o mesmo texto, cinco segundos com o canal de
        // controle aberto ainda garantem que a sessão terminou de inicializar.
        if (!cookieAtualizado && agora - cookieSolicitadoEm >= RESPOSTA_COOKIE_MS) {
            cookieAtualizado = true;
        }

        return cookieAtualizado;
    }

    function limparConexaoParada() {
        if (!window.desktop) {
            return;
        }

        try {
            // Com desktop existente, a função oficial entra no ramo de
            // desconexão, chama Stop() e devolve desktop para null.
            window.connectDesktop(null, 0);
        } catch (erro) {
            registrar('Falha ao limpar a conexão anterior.', erro);
            try {
                window.desktop.Stop();
            } catch (_) {
            }
            try {
                window.desktopNode = null;
                window.desktop = null;
            } catch (_) {
            }
        }
    }

    function prepararNovaTentativa(agora) {
        tentativaIniciadaEm = 0;
        prontoDesde = 0;
        cookieSolicitadoEm = 0;
        cookieAnterior = '';
        cookieAtualizado = false;
        proximaAcaoEm = agora + INTERVALO_REPETICAO_MS;
    }

    function iniciarConexao(agora) {
        tentativas += 1;
        tentativaIniciadaEm = agora;
        atualizarStatus('Conectando automaticamente...');
        registrar('Iniciando tentativa ' + tentativas + ' de ' + MAX_TENTATIVAS + '.');

        try {
            // É o mesmo caminho usado internamente pelo menu do MeshCentral:
            // primeiro consulta as sessões do Windows e então conecta à sessão ativa.
            window.connectDesktop(null, 3);
        } catch (erro) {
            registrar('A chamada de conexão falhou.', erro);
            prepararNovaTentativa(agora);
        }
    }

    function finalizar(mensagem, erro) {
        encerrado = true;
        window.clearInterval(temporizador);
        atualizarStatus(mensagem);
        registrar(mensagem, erro || null);
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

        if (
            tentativas >= MAX_TENTATIVAS &&
            tentativaIniciadaEm === 0 &&
            !conexao
        ) {
            finalizar('Não foi possível conectar automaticamente.');
            return;
        }

        if (conexao) {
            if (
                tentativaIniciadaEm > 0 &&
                agora - tentativaIniciadaEm >= CONEXAO_MS
            ) {
                atualizarStatus('Reconectando automaticamente...');
                registrar('A conexão ficou parada no estado ' + conexao.State + '; limpando.');
                limparConexaoParada();
                prepararNovaTentativa(agora);
            }
            return;
        }

        if (tentativaIniciadaEm > 0) {
            // O modo 3 consulta primeiro as sessões do Windows e pode ainda não
            // ter criado o objeto desktop. Se não houver resposta, recomeça com
            // um cookie de relay novo.
            if (agora - tentativaIniciadaEm >= RESPOSTA_SESSAO_MS) {
                registrar('A consulta das sessões do Windows não respondeu; repetindo.');
                prepararNovaTentativa(agora);
            }
            return;
        }

        if (agora < proximaAcaoEm) {
            return;
        }

        if (!paginaPronta()) {
            prontoDesde = 0;
            return;
        }

        if (!prontoDesde) {
            prontoDesde = agora;
            atualizarStatus('Preparando conexão segura...');
            return;
        }

        if (agora - prontoDesde < ESTABILIDADE_MS) {
            return;
        }

        solicitarCookieAtualizado();

        if (!verificarCookieAtualizado(agora)) {
            return;
        }

        iniciarConexao(agora);
    }, INTERVALO_MS);
})();
