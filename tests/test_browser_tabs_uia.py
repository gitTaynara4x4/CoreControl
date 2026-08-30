from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def test_agent_has_extension_free_browser_tab_fallback():
    source = read("agent/src/browser_tabs_windows.go")
    assert "collectBrowserTabsUIA()" in source
    assert "UIAutomationClient" in source
    assert "UIAutomationTypes" in source
    assert "ControlType]::TabItem" in source
    assert "$process -eq 'chrome'" in source
    assert "$process -eq 'msedge'" in source
    assert "$process -eq 'opera'" in source
    assert 'Source: "windows-uia"' in source
    assert 'Source = "browser-bridge"' in source


def test_bridge_is_preferred_but_uia_fills_missing_browser():
    source = read("agent/src/browser_tabs_windows.go")
    assert "bridgeTabs, bridgeBrowsers := loadBrowserBridgeTabs()" in source
    assert "if browser == \"\" || bridgeBrowsers[browser]" in source
    assert "URL/domain stay empty unless the native browser bridge is installed" in source


def test_panel_lists_all_browser_tabs_with_independent_scroll():
    script = read("app/static/js/pages/devices.js")
    styles = read("app/static/styles.css")
    assert "Abas abertas do navegador" in script
    assert "browserTabs.map((tab) =>" in script
    assert "activity-browser-scroll" in script
    assert ".activity-browser-scroll{max-height:390px;overflow:auto" in styles
    assert "activityBrowserTabsVersionSupported" in script
    assert "missingBrowserTabs" in script


def test_agent_version_bumped_for_browser_tab_fallback():
    main = read("agent/src/main.go")
    assert 'const agentVersion = "0.8.5"' in main
