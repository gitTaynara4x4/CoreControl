from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_custom_js_recovers_target_when_query_is_consumed():
    script = (ROOT / "app" / "remote_assets" / "meshcentral-custom.js").read_text(encoding="utf-8")
    assert "CoreControl Remote v10.7-session-recovery" in script
    assert "param('ctnode') || param('gotonode')" in script
    assert "sessionStorage.getItem('coretuner.remote.node')" in script
    assert "window.__coreControlRemoteVersion = VERSAO" in script
    assert "window.__coreControlRemoteDebug" in script


def test_custom_js_sets_desktop_node_before_connecting():
    script = (ROOT / "app" / "remote_assets" / "meshcentral-custom.js").read_text(encoding="utf-8")
    assert "window.desktopNode = n;" in script
    start = script.index("function iniciarDesktop(n)")
    connect = script.index("window.connectDesktop(null, 1)", start)
    assert script.index("window.desktopNode = n;", start) < connect
