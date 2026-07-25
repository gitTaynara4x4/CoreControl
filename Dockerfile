FROM python:3.13-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    CORETUNER_DATA_DIR=/data \
    CORETUNER_PORT=8280 \
    CORETUNER_REMOTE_MESHCTRL_PATH=/opt/meshcentral-client/node_modules/meshcentral/meshctrl.js

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates nodejs npm \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /opt/meshcentral-client \
    && cd /opt/meshcentral-client \
    && npm init -y >/dev/null 2>&1 \
    && npm install --omit=dev --no-audit --no-fund meshcentral@1.2.4 \
    && npm cache clean --force

COPY requirements.txt ./requirements.txt
RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir -r requirements.txt

COPY app ./app
COPY run.py ./run.py

RUN useradd --system --uid 10001 --create-home coretuner \
    && mkdir -p /data/remote-agents \
    && chown -R coretuner:coretuner /app /data /opt/meshcentral-client

USER coretuner

EXPOSE 8280

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8280/health', timeout=4).read()" || exit 1

CMD ["python", "run.py"]
