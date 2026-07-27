from pathlib import Path


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "app" / "remote_assets" / "meshcentral-custom.js"


def test_remote_asset_uses_direct_meshagent_connection():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "window.meshserver.State === 2" in script
    assert "typeof window.connectDesktop === 'function'" in script
    assert "window.connectDesktop(null, 1)" in script
    assert "window.updateDesktopButtons()" in script
    assert "botao.click()" not in script
    assert "window.connectDesktop(null, 3)" not in script
    assert "window.meshserver.send({ action: 'authcookie' })" not in script


def test_remote_asset_only_runs_for_coretuner_sessions():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "parametros.get('coretuner') === '1'" in script
    assert "window.__coreTunerRemoteAutoStart" in script
    assert "CoreTuner Remote v6" in script


def test_remote_asset_retries_stuck_desktop_connections():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "MAX_TENTATIVAS = 4" in script
    assert "LIMITE_CONEXAO_MS = 20000" in script
    assert "window.desktop.Stop()" in script
