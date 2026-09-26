# Minimal lightweight Dockerfile for Optimus AI Runtime
# TODO: In production, pre-download embedding models and configure GPU drivers for local vLLM/Ollama
FROM python:3.12-slim
WORKDIR /app
COPY pyproject.toml README.md ./
RUN pip install --no-cache-dir fastapi uvicorn httpx pydantic
COPY src/ ./src/
ENV PYTHONPATH=/app/src
EXPOSE 8000
ENTRYPOINT ["uvicorn", "optimus_ai.main:app", "--host", "0.0.0.0", "--port", "8000"]
