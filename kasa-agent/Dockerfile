FROM python:3.13-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Declared after the pip layer so changing the version doesn't bust its cache.
ARG VERSION=unknown
ARG REVISION=unknown
ENV KASA_AGENT_VERSION=$VERSION \
    KASA_AGENT_REVISION=$REVISION

COPY app.py .

EXPOSE 8090

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8090"]
