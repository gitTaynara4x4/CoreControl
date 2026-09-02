from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_agent_exposes_remote_optimization_command():
    source = (ROOT / "agent" / "src" / "update_windows.go").read_text(encoding="utf-8")
    optimizer = (ROOT / "agent" / "src" / "optimization_windows.go").read_text(encoding="utf-8")
    main = (ROOT / "agent" / "src" / "main.go").read_text(encoding="utf-8")
    assert 'case "optimization.apply"' in source
    assert "applyOptimizationProfile(payload.Profile)" in source
    assert 'const agentVersion = "0.9.0"' in main
    assert "optimization-state.json" in optimizer
    assert "backup automático" in optimizer


def test_remote_optimizer_keeps_safety_guards():
    optimizer = (ROOT / "agent" / "src" / "optimization_windows.go").read_text(encoding="utf-8").lower()
    for forbidden in ["taskkill", "remove-item", "disable firewall", "windows defender", "format.com"]:
        assert forbidden not in optimizer
    assert "optnormalpriority" not in optimizer
    assert "optabovenormalpriority" in optimizer
    assert "a restauração ficou incompleta; o backup foi mantido" in optimizer


def test_panel_has_optimization_card_and_api_calls():
    page = (ROOT / "app" / "static" / "pages" / "device.html").read_text(encoding="utf-8")
    js = (ROOT / "app" / "static" / "js" / "pages" / "devices.js").read_text(encoding="utf-8")
    api = (ROOT / "app" / "update_api.py").read_text(encoding="utf-8")
    assert 'id="optimizationProfiles"' in page
    assert '`/devices/${device.id}/optimization`' in js
    assert '"optimization.apply"' in api
    assert '"company_admin"' in api
    assert "0, 9, 0" in api


def test_v107_cache_bust_is_present():
    html = (ROOT / "app" / "static" / "index.html").read_text(encoding="utf-8")
    assert "optimization-remote-v10-7" in html
