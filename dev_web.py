from __future__ import annotations

import os

import uvicorn
from dotenv import load_dotenv

load_dotenv()

if __name__ == "__main__":
    # Modo de desenvolvimento visual: navegador + reload do servidor.
    os.environ.setdefault("CORETUNER_ENV", "development")
    os.environ["CORETUNER_DEV_WEB"] = "1"
    os.environ["CORETUNER_PUBLIC_URL"] = "http://127.0.0.1:8001"

    uvicorn.run(
        "app.main:app",
        host="127.0.0.1",
        port=8001,
        reload=True,
        proxy_headers=True,
        forwarded_allow_ips="*",
    )
