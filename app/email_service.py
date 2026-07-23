from __future__ import annotations

import html
import smtplib
import ssl
from email.message import EmailMessage
from email.utils import formataddr

from .config import settings


class EmailDeliveryError(RuntimeError):
    """Falha controlada ao entregar uma mensagem de e-mail."""


def send_password_reset_email(*, recipient_name: str, recipient_email: str, reset_url: str) -> None:
    if not settings.smtp_configured:
        raise EmailDeliveryError("O envio de e-mail ainda não foi configurado no servidor.")

    safe_name = html.escape(recipient_name or "usuário")
    safe_url = html.escape(reset_url, quote=True)
    expires_minutes = settings.password_reset_minutes

    message = EmailMessage()
    message["Subject"] = "Redefinição de senha | CoreTuner"
    message["From"] = formataddr((settings.smtp_from_name, settings.smtp_sender))
    message["To"] = recipient_email
    message.set_content(
        f"Olá, {recipient_name or 'usuário'}.\n\n"
        "Recebemos uma solicitação para redefinir sua senha do CoreTuner.\n"
        f"Use o link abaixo em até {expires_minutes} minutos:\n\n"
        f"{reset_url}\n\n"
        "Caso você não tenha solicitado a alteração, ignore esta mensagem. "
        "Sua senha continuará a mesma.\n"
    )
    message.add_alternative(
        f"""
        <!doctype html>
        <html lang="pt-BR">
          <body style="margin:0;background:#f4f7fb;font-family:Arial,sans-serif;color:#10213d">
            <div style="max-width:580px;margin:0 auto;padding:32px 18px">
              <div style="background:#ffffff;border:1px solid #dce5f0;border-radius:18px;padding:34px;box-shadow:0 10px 35px rgba(15,36,72,.08)">
                <div style="font-size:24px;font-weight:800;margin-bottom:24px">Core<span style="color:#1467f4">Tuner</span></div>
                <h1 style="font-size:24px;margin:0 0 14px">Redefinir sua senha</h1>
                <p style="font-size:15px;line-height:1.6;color:#5e6f88">Olá, {safe_name}. Recebemos uma solicitação para redefinir sua senha.</p>
                <p style="font-size:15px;line-height:1.6;color:#5e6f88">O botão abaixo ficará disponível por <strong>{expires_minutes} minutos</strong> e poderá ser usado uma única vez.</p>
                <p style="margin:28px 0">
                  <a href="{safe_url}" style="display:inline-block;background:#1467f4;color:#ffffff;text-decoration:none;font-weight:700;padding:14px 22px;border-radius:10px">Criar nova senha</a>
                </p>
                <p style="font-size:12px;line-height:1.55;color:#8190a5;word-break:break-all">Se o botão não abrir, copie este endereço:<br>{safe_url}</p>
                <hr style="border:0;border-top:1px solid #e4eaf2;margin:26px 0">
                <p style="font-size:12px;line-height:1.55;color:#8190a5">Caso você não tenha solicitado a alteração, ignore esta mensagem. Sua senha continuará a mesma.</p>
              </div>
            </div>
          </body>
        </html>
        """,
        subtype="html",
    )

    try:
        with smtplib.SMTP(settings.smtp_host, settings.smtp_port, timeout=25) as smtp:
            smtp.ehlo()
            if settings.smtp_starttls:
                smtp.starttls(context=ssl.create_default_context())
                smtp.ehlo()
            smtp.login(settings.smtp_user, settings.smtp_password)
            smtp.send_message(message)
    except (OSError, smtplib.SMTPException) as exc:
        raise EmailDeliveryError("Não foi possível entregar o e-mail de recuperação.") from exc
