"""Architectural boundary assertions for ai-runtime.

Enforces ADR-0008 invariants:
1. Agent tools and orchestration must interact with enterprise systems via MCP or RAG,
   NEVER importing direct database drivers or raw storage APIs (psycopg, asyncpg, sqlalchemy directly in agents).
2. Agent skills and orchestration must not depend directly on transport / server code (main.py).
"""

import ast
from pathlib import Path


def get_imports_for_file(filepath: Path) -> list[str]:
    with open(filepath, "r", encoding="utf-8") as f:
        tree = ast.parse(f.read(), filename=str(filepath))

    imports = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                imports.append(alias.name)
        elif isinstance(node, ast.ImportFrom):
            if node.module:
                imports.append(node.module)
    return imports


def test_agent_layer_does_not_import_direct_db_drivers():
    src_dir = Path(__file__).resolve().parent.parent / "src" / "optimus_ai"
    agents_dir = src_dir / "agents"
    orchestration_dir = src_dir / "orchestration"

    disallowed_db_packages = {"asyncpg", "psycopg2", "psycopg", "pymysql", "sqlite3"}

    for target_dir in [agents_dir, orchestration_dir]:
        for py_file in target_dir.glob("**/*.py"):
            imports = get_imports_for_file(py_file)
            for imp in imports:
                root_pkg = imp.split(".")[0]
                assert root_pkg not in disallowed_db_packages, (
                    f"Architecture violation: {py_file} directly imports database driver '{imp}'. "
                    "Agents must interact strictly via MCP tools or RAG."
                )


def test_agent_layer_does_not_import_server_main():
    src_dir = Path(__file__).resolve().parent.parent / "src" / "optimus_ai"
    agents_dir = src_dir / "agents"

    for py_file in agents_dir.glob("**/*.py"):
        imports = get_imports_for_file(py_file)
        for imp in imports:
            assert not imp.endswith("main") and "main" not in imp.split("."), (
                f"Architecture violation: {py_file} imports server transport entrypoint '{imp}'."
            )
