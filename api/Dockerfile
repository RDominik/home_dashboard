FROM python:3.11-slim

# git + Docker CLI für System-Update
RUN apt-get update && apt-get install -y --no-install-recommends \
      git curl ca-certificates gnupg && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian bookworm stable" \
      > /etc/apt/sources.list.d/docker.list && \
    apt-get update && apt-get install -y --no-install-recommends docker-ce-cli docker-compose-plugin && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# git safe directory für gemountetes Repo
RUN git config --global --add safe.directory /repo

WORKDIR /app
COPY api/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt
COPY api /app/api
EXPOSE 8083
CMD ["uvicorn", "api.main:app", "--host", "0.0.0.0", "--port", "8083"]
