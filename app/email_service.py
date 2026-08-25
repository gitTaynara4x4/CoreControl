from __future__ import annotations

import logging
import smtplib
import ssl
from email.message import EmailMessage
from email.policy import SMTP
from email.utils import formatdate, make_msgid

from .config import settings


logger = logging.getLogger(__name__)


class EmailDeliveryError(RuntimeError):
    """Falha controlada ao entregar uma mensagem de e-mail."""


def _mask_email(value: str) -> str:
    local, separator, domain = value.partition("@")
    if not separator:
        return "***"
    visible = local[:2] if len(local) > 2 else local[:1]
    return f"{visible}***@{domain}"


def send_password_reset_email(*, recipient_name: str, recipient_email: str, reset_url: str) -> None:
    """Envia a recuperação em texto simples para maximizar a entrega.

    A versão anterior usava uma mensagem HTML elaborada. O Gmail aceitava a
    mensagem no SMTP, mas podia retê-la silenciosamente pelos filtros
    antiphishing. Esta versão usa o mesmo formato simples que já foi confirmado
    como entregue no servidor de produção.
    """
    if not settings.smtp_configured:
        raise EmailDeliveryError("O envio de e-mail ainda não foi configurado no servidor.")

    recipient = recipient_email.strip()
    if not recipient or "@" not in recipient:
        raise EmailDeliveryError("O endereço de e-mail do destinatário é inválido.")

    display_name = (recipient_name or "usuário").strip() or "usuário"
    expires_minutes = settings.password_reset_minutes

    message = EmailMessage(policy=SMTP)
    message["Subject"] = "CoreControl - link para criar nova senha"
    # Mantém o remetente exatamente igual à conta autenticada no SMTP.
    message["From"] = settings.smtp_sender
    message["To"] = recipient
    message["Reply-To"] = settings.smtp_sender
    message["Date"] = formatdate(localtime=False, usegmt=True)
    message_id = make_msgid(domain=settings.smtp_sender.partition("@")[2] or None)
    message["Message-ID"] = message_id
    message["Auto-Submitted"] = "auto-generated"
    message["X-Auto-Response-Suppress"] = "All"
    message.set_content(
        f"Olá, {display_name}.\n\n"
        "Recebemos uma solicitação para criar uma nova senha da sua conta CoreControl.\n\n"
        f"Abra este endereço em até {expires_minutes} minutos:\n"
        f"{reset_url}\n\n"
        "O link funciona uma única vez.\n\n"
        "Caso você não tenha solicitado esta alteração, ignore esta mensagem. "
        "Sua senha atual continuará funcionando.\n\n"
        "Equipe CoreControl\n"
    )

    try:
        with smtplib.SMTP(settings.smtp_host, settings.smtp_port, timeout=30) as smtp:
            smtp.ehlo()
            if settings.smtp_starttls:
                smtp.starttls(context=ssl.create_default_context())
                smtp.ehlo()

            smtp.login(settings.smtp_user, settings.smtp_password)
            refused = smtp.send_message(
                message,
                from_addr=settings.smtp_user,
                to_addrs=[recipient],
            )

            if refused:
                raise EmailDeliveryError(
                    "O servidor SMTP recusou o destinatário do e-mail de recuperação."
                )

        logger.info(
            "E-mail de recuperação aceito pelo SMTP: destinatário=%s message_id=%s",
            _mask_email(recipient),
            message_id,
        )
    except EmailDeliveryError:
        raise
    except (OSError, smtplib.SMTPException) as exc:
        raise EmailDeliveryError("Não foi possível entregar o e-mail de recuperação.") from exc
