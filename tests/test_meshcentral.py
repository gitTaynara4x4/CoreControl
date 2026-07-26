import base64
from urllib.parse import parse_qs, urlparse
from types import SimpleNamespace

from app.meshcentral import (
    MeshCentralClient,
    MeshDevice,
    _mesh_id_for_download,
    _mesh_id_to_hex,
    build_remote_desktop_url,
)


def test_mesh_id_to_hex_accepts_meshcentral_base64_identifier():
    raw = bytes(range(32))
    encoded = base64.b64encode(raw).decode("ascii").rstrip("=")
    mesh_id = f"mesh//{encoded}"
    assert _mesh_id_to_hex(mesh_id) == raw.hex()




def test_mesh_id_to_hex_accepts_bare_hex_identifier_from_meshctrl_hex_mode():
    raw_hex = bytes(range(32)).hex()
    assert _mesh_id_to_hex(raw_hex) == raw_hex


def test_mesh_id_to_hex_accepts_prefixed_hex_identifier():
    raw_hex = bytes(reversed(range(32))).hex()
    assert _mesh_id_to_hex(f"mesh//{raw_hex}") == raw_hex


def test_mesh_id_to_hex_accepts_current_48_byte_meshcentral_identifier():
    raw = bytes(range(48))
    encoded = base64.b64encode(raw).decode("ascii").rstrip("=")
    mesh_id = f"mesh//{encoded}"
    assert _mesh_id_to_hex(mesh_id) == raw.hex()


def test_mesh_id_to_hex_accepts_current_96_character_hex_identifier():
    raw_hex = bytes(range(48)).hex()
    assert _mesh_id_to_hex(f"0x{raw_hex.upper()}") == raw_hex


def test_remote_url_targets_exact_node():
    url = build_remote_desktop_url(
        base_url="https://remote.example.com",
        login_token="TOKEN",
        node_id="node//NODE123",
    )
    query = parse_qs(urlparse(url).query)
    assert query["login"] == ["TOKEN"]
    assert query["gotonode"] == ["NODE123"]
    assert query["viewmode"] == ["11"]
    assert query["hide"] == ["63"]
    assert query["coretuner"] == ["1"]


def test_match_device_uses_saved_node_then_unique_hostname():
    client = MeshCentralClient()
    devices = [
        MeshDevice(
            node_id="node//A",
            mesh_id="mesh//G",
            name="PC SALA",
            real_name="",
            hostname="DESKTOP-A",
            connected=True,
            raw={},
        ),
        MeshDevice(
            node_id="node//B",
            mesh_id="mesh//G",
            name="OUTRO",
            real_name="",
            hostname="DESKTOP-B",
            connected=False,
            raw={},
        ),
    ]
    assert client.match_device(SimpleNamespace(mesh_node_id="node//B", hostname="x", name="x"), devices).node_id == "node//B"
    assert client.match_device(SimpleNamespace(mesh_node_id=None, hostname="desktop-a", name="irrelevante"), devices).node_id == "node//A"


def test_ensure_company_group_accepts_96_character_meshctrl_idhex(monkeypatch):
    client = MeshCentralClient()
    raw_hex = bytes(range(48)).hex()
    integration_user_id = "user//coretuner-integracao"
    group = {
        "_id": "mesh//IDENTIFICADOR",
        "_idhex": raw_hex,
        "name": "CoreTuner - Empresa [empresa-1]",
        "links": {integration_user_id: {"rights": 8}},
    }
    monkeypatch.setattr(client, "_list_groups", lambda: [group])
    monkeypatch.setattr(client, "ensure_integration_user", lambda: integration_user_id)

    company = SimpleNamespace(
        id=1,
        name="Empresa",
        slug="empresa",
        mesh_group_id="mesh//IDENTIFICADOR",
    )
    mesh_id, mesh_hex, group_name = client.ensure_company_group(company)

    assert mesh_id == "mesh//IDENTIFICADOR"
    assert mesh_hex == raw_hex
    assert group_name == group["name"]


def test_mesh_id_for_download_preserves_meshcentral_modified_base64_identifier():
    raw = bytes(range(48))
    encoded = base64.b64encode(raw).decode("ascii").rstrip("=").replace("+", "@").replace("/", "$")
    assert _mesh_id_for_download(f"mesh//{encoded}") == encoded


def test_mesh_id_for_download_converts_96_character_hex_to_modified_base64():
    raw = bytes(range(48))
    expected = base64.b64encode(raw).decode("ascii").rstrip("=").replace("+", "@").replace("/", "$")
    assert _mesh_id_for_download(raw.hex()) == expected


def test_prepare_company_agent_downloads_with_original_mesh_id_not_hex(monkeypatch, tmp_path):
    client = MeshCentralClient()
    raw = bytes(range(48))
    encoded = base64.b64encode(raw).decode("ascii").rstrip("=").replace("+", "@").replace("/", "$")
    mesh_id = f"mesh//{encoded}"
    mesh_hex = raw.hex()
    company = SimpleNamespace(id=1, name="Empresa", slug="empresa")
    captured = {}

    monkeypatch.setattr(client, "ensure_company_group", lambda company: (mesh_id, mesh_hex, "Grupo"))
    monkeypatch.setattr(client, "_agent_path", lambda value: tmp_path / "CoreTunerRemoteAgent.exe")

    def fake_download(value, target):
        captured["mesh_id"] = value
        target.write_bytes(b"MZ" + (b"0" * 100_000))

    monkeypatch.setattr(client, "_download_agent", fake_download)
    prepared = client.prepare_company_agent(company)

    assert captured["mesh_id"] == mesh_id
    assert prepared.mesh_group_hex == mesh_hex
