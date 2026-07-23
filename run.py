from __future__ import annotations

import os

import uvicorn
from dotenv import load_dotenv

load_dotenv()


if __name__ == "__main__":
    port = int(os.getenv("PORT", os.getenv("CORETUNER_PORT", "8002")))
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=port,
        reload=False,
        proxy_headers=True,
        forwarded_allow_ips="*",
    )
