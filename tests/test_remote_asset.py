from pathlib import Path


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "app" / "remote_assets" / "meshcentral-custom.js"


def test_remote_asset_restores_exact_current_node_before_connecting():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "obterParametro('ctnode')" in script
    assert "coretuner.remote.node" in script
    assert "window.currentNode = computador" in script
    assert "window.gotoDevice(computador._id, 11)" in script
    assert "window.currentNode = dispositivoSelecionado" in script
    assert "window.connectDesktop(null, 1)" in script


def test_remote_asset_never_uses_first_device_when_multiple_are_visible():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "if (!alvoCurto && lista.length === 1)" in script
    assert "lista.length === 1" in script
    assert "lista[0]" in script
    assert "String(lista[i]._id) === String(nodeAlvo)" in script
    assert "idCurto(lista[i]._id) === alvoCurto" in script


def test_remote_asset_only_runs_for_coretuner_sessions():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "obterParametro('coretuner') === '1'" in script
    assert "window.__coreTunerRemoteAutoStart" in script
    assert "CoreControl Remote v7" in script


def test_remote_asset_retries_stuck_desktop_connections():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "MAX_TENTATIVAS = 4" in script
    assert "LIMITE_CONEXAO_MS = 20000" in script
    assert "window.desktop.Stop()" in script
    assert "window.connectDesktop(null, 3)" not in script
    assert "window.meshserver.send({ action: 'authcookie' })" not in script
