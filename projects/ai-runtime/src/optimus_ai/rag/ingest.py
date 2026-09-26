from __future__ import annotations

import argparse
import asyncio
import json
import logging
import re
from pathlib import Path

from .embeddings import compute_embedding
from .retriever import HybridRetriever

logging.basicConfig(level=logging.INFO, format="[%(asctime)s] %(levelname)s: %(message)s")
logger = logging.getLogger("optimus-ingest")


def chunk_text(text: str, chunk_size: int = 512, chunk_overlap: int = 64) -> list[str]:
    """Chunk document text into overlapping windows while respecting paragraph/sentence boundaries."""
    if len(text) <= chunk_size:
        return [text]

    chunks: list[str] = []
    # Split by double newline or sentence boundaries
    paragraphs = [p.strip() for p in re.split(r"\n\s*\n", text) if p.strip()]

    current_chunk = ""
    for para in paragraphs:
        if len(current_chunk) + len(para) + 2 <= chunk_size:
            current_chunk = f"{current_chunk}\n\n{para}".strip()
        else:
            if current_chunk:
                chunks.append(current_chunk)
            # If a single paragraph is longer than chunk_size, split by sentences
            if len(para) > chunk_size:
                sentences = re.split(r"(?<=[.!?])\s+", para)
                sub_chunk = ""
                for sent in sentences:
                    if len(sub_chunk) + len(sent) + 1 <= chunk_size:
                        sub_chunk = f"{sub_chunk} {sent}".strip()
                    else:
                        if sub_chunk:
                            chunks.append(sub_chunk)
                        sub_chunk = sent
                if sub_chunk:
                    chunks.append(sub_chunk)
                current_chunk = ""
            else:
                current_chunk = para

    if current_chunk:
        chunks.append(current_chunk)

    return chunks


async def ingest_document_file(
    retriever: HybridRetriever,
    tenant_id: str,
    file_path: Path,
    chunk_size: int = 512,
    chunk_overlap: int = 64,
) -> int:
    """Read a document file (JSON or text), chunk it, generate embeddings, and persist to database."""
    content = ""
    title = file_path.stem
    doc_id = file_path.stem.upper()
    doc_type = "MAINTENANCE_MANUAL"

    if file_path.suffix == ".json":
        data = json.loads(file_path.read_text(encoding="utf-8"))
        doc_id = data.get("document_id", data.get("id", doc_id))
        title = data.get("title", title)
        doc_type = data.get("doc_type", doc_type)
        content = data.get("content", data.get("text", ""))
        # If chunks are already provided in JSON fixture
        if "chunks" in data and isinstance(data["chunks"], list):
            chunks = [c.get("content", str(c)) for c in data["chunks"]]
        else:
            chunks = chunk_text(content, chunk_size=chunk_size, chunk_overlap=chunk_overlap)
    else:
        content = file_path.read_text(encoding="utf-8")
        chunks = chunk_text(content, chunk_size=chunk_size, chunk_overlap=chunk_overlap)

    if not chunks:
        chunks = [content]

    logger.info("Ingesting document %s ('%s') into %d chunks for tenant %s...", doc_id, title, len(chunks), tenant_id)

    # Ingest main document header
    pool = await retriever._get_pool()
    if pool is not None:
        async with pool.acquire() as conn:
            async with conn.transaction():
                await conn.execute("SET LOCAL app.current_tenant = $1", tenant_id)
                await conn.execute(
                    """
                    INSERT INTO documents (id, tenant_id, title, doc_type)
                    VALUES ($1, $2, $3, $4)
                    ON CONFLICT (tenant_id, id) DO UPDATE SET title = EXCLUDED.title;
                    """,
                    doc_id,
                    tenant_id,
                    title,
                    doc_type,
                )

                # Delete old chunks if re-indexing
                await conn.execute(
                    "DELETE FROM document_chunks WHERE tenant_id = $1 AND document_id = $2",
                    tenant_id,
                    doc_id,
                )

                for idx, chunk in enumerate(chunks):
                    embedding = compute_embedding(chunk)
                    await conn.execute(
                        """
                        INSERT INTO document_chunks (tenant_id, document_id, chunk_index, content, embedding)
                        VALUES ($1, $2, $3, $4, $5);
                        """,
                        tenant_id,
                        doc_id,
                        idx,
                        chunk,
                        embedding,
                    )
    else:
        # Standalone in-memory fallback
        for chunk in chunks:
            await retriever.ingest_document(
                tenant_id=tenant_id,
                document_id=doc_id,
                title=title,
                content=chunk,
                doc_type=doc_type,
            )

    logger.info("Successfully indexed %d chunks for document %s", len(chunks), doc_id)
    return len(chunks)


async def main() -> None:
    parser = argparse.ArgumentParser(description="Optimus Knowledge Ingestion Worker")
    parser.add_argument("--tenant", default="acme", help="Tenant ID")
    parser.add_argument("--chunk-size", type=int, default=512, help="Chunk token/char size")
    parser.add_argument("--chunk-overlap", type=int, default=64, help="Chunk overlap")
    parser.add_argument("--file", help="Path to document file to ingest")
    parser.add_argument("--dir", help="Directory of document files to ingest")
    args = parser.parse_args()

    retriever = HybridRetriever()
    total_indexed = 0

    if args.file:
        p = Path(args.file)
        if p.exists():
            total_indexed += await ingest_document_file(
                retriever, args.tenant, p, chunk_size=args.chunk_size, chunk_overlap=args.chunk_overlap
            )
    elif args.dir:
        d = Path(args.dir)
        if d.exists():
            for f in d.glob("*"):
                if f.is_file() and f.suffix in [".json", ".txt", ".md"]:
                    total_indexed += await ingest_document_file(
                        retriever, args.tenant, f, chunk_size=args.chunk_size, chunk_overlap=args.chunk_overlap
                    )
    else:
        # Default: Ingest primary PLM manual fixture PLM-COOL-4021
        logger.info("No file or directory specified. Ingesting primary PLM cooling manual fixture...")
        default_content = (
            "Centrifugal Pump Cooling Loop Maintenance Manual (§4.2 Troubleshooting). "
            "Recurrent overheating in pump cooling manifolds (such as Manchester Pump P-104) "
            "typically indicates thermostat bypass valve sticking or heat exchanger tube fouling. "
            "Primary corrective action requires thermostat replacement with part SP-COOL-9981. "
            "Secondary cooling circuit flush must be executed to clear thermal debris."
        )
        sample_path = Path("/tmp/PLM-COOL-4021.txt")
        sample_path.write_text(default_content, encoding="utf-8")
        total_indexed += await ingest_document_file(
            retriever,
            args.tenant,
            sample_path,
            chunk_size=args.chunk_size,
            chunk_overlap=args.chunk_overlap,
        )

    logger.info("Optimus ingestion complete. Total document chunks indexed: %d", total_indexed)


if __name__ == "__main__":
    asyncio.run(main())
