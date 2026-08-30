from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_agent_snapshot_carries_cached_executable_icons():
    activity = read("agent/src/activity_windows.go")
    icons = read("agent/src/activity_icons_windows.go")
    assert 'AppAssets   map[string]activityAppAsset `json:"app_assets,omitempty"`' in activity
    assert "SHGetFileInfoW" in icons
    assert "SendMessageTimeoutW" in icons
    assert "GetClassLongPtrW" in icons
    assert "ExtractIconExW" in icons
    assert "CreateDIBSection" in icons
    assert '"data:image/png;base64,"' in icons
    assert "activityIconCache" in icons


def test_agent_version_bumped_for_activity_icon_support():
    main = read("agent/src/main.go")
    assert 'const agentVersion = "0.8.1"' in main


def test_device_panel_renders_real_icons_and_friendly_names():
    script = read("app/static/js/pages/devices.js")
    styles = read("app/static/styles.css")
    assert "activityRememberAssets" in script
    assert "activityAppIcon" in script
    assert "Google Chrome" in script
    assert "Configurações" in script
    assert "Microsoft Text Input" in script
    assert ".activity-app-icon img" in styles
    assert ".activity-app-icon.focused::after" in styles
