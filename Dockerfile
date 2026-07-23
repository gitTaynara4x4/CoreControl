FROM python:3.13-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    CORETUNER_DATA_DIR=/data \
    CORETUNER_PORT=8280

WORKDIR /app

COPY requirements.txt ./requirements.txt
RUN pip install --no-cache-dir --upgrade pip \
    && pip install --no-cache-dir -r requirements.txt

COPY app ./app
COPY run.py ./run.py

RUN useradd --system --uid 10001 --create-home coretuner \
    && mkdir -p /data \
    && chown -R coretuner:coretuner /app /data

USER coretuner

EXPOSE 8280

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8280/health', timeout=4).read()" || exit 1

CMD ["python", "run.py"]
