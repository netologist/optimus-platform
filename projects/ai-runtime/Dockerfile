# Minimal lightweight Dockerfile for Optimus AI Runtime
# TODO: In production, pre-download embedding models and configure GPU drivers for local vLLM/Ollama
FROM python:3.12-slim
WORKDIR /app
COPY pyproject.toml README.md ./
RUN pip install --no-cache-dir "pydantic>=2.10.0" "pydantic-ai>=0.0.18" "anthropic>=0.40.0" "openai>=1.50.0" "fastapi>=0.115.0" "uvicorn>=0.34.0" "httpx>=0.28.0" "numpy>=2.0.0" "asyncpg>=0.30.0" "pgvector>=0.3.6"
COPY src/ ./src/
RUN pip install --no-cache-dir --no-deps -e .
ENV PYTHONPATH=/app/src
EXPOSE 8000
ENTRYPOINT ["uvicorn", "optimus_ai.main:app", "--host", "0.0.0.0", "--port", "8000"]
