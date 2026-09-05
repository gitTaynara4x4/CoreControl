from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "app" / "remote_assets" / "meshcentral-custom.js"


def read_script():
    return SCRIPT.read_text(encoding="utf-8")


def test_remote_v108_binds_saved_node_to_desktop():
    script = read_script()
    assert "CoreControl Remote v10.8-node-bind" in script
    assert "window.desktopNode = n;" in script
    assert "window.currentNode = n;" in script
    assert "document.getElementById('connectbutton1')" in script
    assert "botao.disabled = false" in script


def test_remote_v108_uses_mesh_agent_desktop_mode_3():
    script = read_script()
    assert "window.connectDesktop(null, 3)" in script
    assert "window.connectDesktop(null, 1)" not in script


def test_remote_v108_does_not_gate_on_xxcurrentview():
    script = read_script()
    start = script.index("function prontoParaConectar")
    end = script.index("function fixarNodeNoDesktop", start)
    body = script[start:end]
    assert "xxcurrentView" not in body


def test_remote_v108_has_version_specific_guard():
    script = read_script()
    assert "window.__coreTunerRemoteAutoStartV108" in script
