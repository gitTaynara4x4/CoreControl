from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
APP_SRC = ROOT / "desktop" / "app" / "src"


def read(name: str) -> str:
    return (APP_SRC / name).read_text(encoding="utf-8")


def test_real_optimizer_has_persistent_backup_before_changes():
    source = read("optimization_windows.go")
    assert "createOptimizationBaseline" in source
    assert "saveOptimizationState(path, state)" in source
    baseline_pos = source.index("state, err = createOptimizationBaseline(path)")
    first_animation_change = source.index("setMinimizeAnimation", baseline_pos)
    assert baseline_pos < first_animation_change


def test_optimizer_never_uses_destructive_commands():
    source = (read("optimization_windows.go") + read("optimizer_core.go")).lower()
    forbidden = [
        "taskkill",
        "remove-item",
        "empty recycle",
        "windows defender",
        "disable firewall",
        "reg delete",
        "del /",
        "format.com",
    ]
    for token in forbidden:
        assert token not in source


def test_optimizer_uses_only_moderate_process_priority():
    source = read("optimization_windows.go")
    assert "aboveNormalPriority" in source
    assert "HIGH_PRIORITY_CLASS" not in source
    assert "REALTIME_PRIORITY_CLASS" not in source


def test_restore_keeps_backup_when_incomplete():
    source = read("optimization_windows.go")
    assert "a restauração ficou incompleta; o backup foi mantido" in source
    assert "archiveOptimizationState" in source


def test_desktop_manifest_matches_optimizer_release():
    api = (ROOT / "app" / "api.py").read_text(encoding="utf-8")
    assert 'return {"version": "0.4.14", "files": files}' in api


def test_optimization_screen_explains_each_profile_in_plain_language():
    main = read("main.go")
    core = read("optimizer_core.go")

    assert '"O que o perfil "+detail.Name+" fará"' in main
    assert '"Proteções em todos os perfis"' in main
    assert '"Ver e selecionar"' in main
    assert '"Ajustes seguros"' not in main
    assert '"Bloqueado por segurança"' not in main

    required_customer_texts = [
        "Deixa o Windows mais leve sem mudar energia ou programas.",
        "Reduz efeitos visuais e mantém desempenho e consumo equilibrados.",
        "Prepara o PC para navegador, WhatsApp, CRM e discador.",
        "Na tomada, ativa Alto desempenho; na bateria ou sem confirmação de energia, usa Equilibrado.",
        "Desfaz as mudanças do CoreTuner e volta ao estado salvo.",
        "Nenhum arquivo ou pasta é apagado ou movido.",
        "Sem backup seguro, nenhuma alteração é iniciada.",
    ]
    combined = main + core
    for text in required_customer_texts:
        assert text in combined
