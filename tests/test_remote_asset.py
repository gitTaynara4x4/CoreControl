from pathlib import Path


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "app" / "remote_assets" / "meshcentral-custom.js"


def test_remote_asset_waits_for_authenticated_control_channel():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "window.meshserver.State === 2" in script
    assert "window.meshserver.send({ action: 'authcookie' })" in script
    assert "window.authRelayCookie" in script
    assert "window.connectDesktop(null, 3)" in script


def test_remote_asset_only_runs_for_coretuner_sessions():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "parametros.get('coretuner') === '1'" in script
    assert "window.__coreTunerRemoteAutoStart" in script


def test_remote_asset_retries_stuck_desktop_connections():
    script = SCRIPT_PATH.read_text(encoding="utf-8")

    assert "window.connectDesktop(null, 0)" in script
    assert "MAX_TENTATIVAS = 5" in script
    assert "CONEXAO_MS = 25000" in script
