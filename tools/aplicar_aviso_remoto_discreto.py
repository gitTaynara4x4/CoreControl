from app.db import SessionLocal
from app.meshcentral import meshcentral_client
from app.models import Company
from app.config import settings


def main() -> None:
    print("CoreControl - aviso remoto discreto")
    print("CORETUNER_REMOTE_GROUP_CONSENT =", settings.remote_group_consent)
    if int(settings.remote_group_consent) != 1:
        raise SystemExit(
            "ERRO: configure CORETUNER_REMOTE_GROUP_CONSENT=1 no EasyPanel e reinicie o CoreControl antes de rodar este comando."
        )

    with SessionLocal() as db:
        companies = db.query(Company).order_by(Company.id).all()
        if not companies:
            print("Nenhuma empresa encontrada.")
            return

        ok = 0
        for company in companies:
            try:
                mesh_id, _mesh_hex, group_name = meshcentral_client.ensure_company_group(company)
                print(f"[OK] {company.name}: grupo consent=1 | {group_name}")

                devices = meshcentral_client.list_group_devices(mesh_id, force=True)
                for device in devices:
                    if not device.node_id:
                        continue
                    meshcentral_client._meshctrl_command(
                        "EditDevice",
                        ["--id", device.node_id, "--consent", "1"],
                    )
                    print(f"     [OK] dispositivo: {device.name or device.hostname or device.node_id[:24]} | consent=1")

                ok += 1
            except Exception as exc:
                print(f"[ERRO] {company.name}: {exc}")

        print(f"Concluido: {ok}/{len(companies)} empresa(s) atualizada(s).")


if __name__ == "__main__":
    main()
