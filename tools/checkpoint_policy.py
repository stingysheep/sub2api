"""Shared allowlist for private, local source checkpoints."""
from pathlib import Path, PurePosixPath

ROOT_FILES = {"Dockerfile", ".dockerignore", "Makefile", "AGENTS.md", "go.mod", "go.sum"}
BACKEND_SOURCE_DIRS = {"backend/cmd", "backend/ent", "backend/internal"}
FRONTEND_SOURCE_DIRS = {"frontend/src", "frontend/public"}
FRONTEND_EXTENSIONS = {".ts", ".tsx", ".vue", ".js", ".jsx", ".css", ".scss", ".sass", ".less", ".html", ".json", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".gif", ".ico", ".woff", ".woff2", ".ttf"}
EXACT_FILES = {
    "backend/go.mod", "backend/go.sum", "frontend/package.json", "frontend/pnpm-lock.yaml",
    "frontend/vite.config.ts", "frontend/tsconfig.node.json", "tools/local-demo/mock-server.mjs",
    "frontend/scripts/operator-browser-smoke.mjs", "frontend/scripts/operator-browser.config.mjs",
    "scripts/deploy-codex-20260830-r3.sh", "docs/architecture/module-boundaries.md",
    "notes/improvement-backlog.md", "notes/model-routing-efficiency.md",
    "notes/autonomous-backlog-contract.md", "notes/billing-recovery-plan.md",
    "notes/local-release-safety.md",
}
SENSITIVE_PARTS = {".git", ".local", ".pnpm-store", ".tmp-go", ".tmp-go27", "data", "node_modules", "dist", "coverage", "secrets", "credentials", ".gocache", "cache", "caches", "tmp", "vendor", "work", "__pycache__"}
SENSITIVE_SUFFIXES = {".env", ".pem", ".key", ".dump"}


def checkpoint_path_allowed(name: str) -> bool:
    """Accept only normalized relative paths covered by the recovery allowlist."""
    if not isinstance(name, str) or not name or "\\" in name or "\x00" in name or ":" in name:
        return False
    path = PurePosixPath(name)
    if name != path.as_posix() or path.is_absolute() or any(part in {"", ".", ".."} for part in path.parts):
        return False
    lower_parts = [part.casefold() for part in path.parts]
    filename = lower_parts[-1]
    if any(part in SENSITIVE_PARTS for part in lower_parts):
        return False
    if filename.startswith(".env") or "production-snapshot" in filename:
        return False
    if PurePosixPath(filename).suffix in SENSITIVE_SUFFIXES:
        return False
    if name in ROOT_FILES or name in EXACT_FILES:
        return True
    parent = path.parent.as_posix()
    if any(parent == directory or parent.startswith(directory + "/") for directory in BACKEND_SOURCE_DIRS):
        return path.suffix.casefold() == ".go"
    if parent == "backend/migrations":
        return path.suffix.casefold() == ".sql"
    if parent == "backend/resources" or parent.startswith("backend/resources/"):
        return path.suffix.casefold() in {".json", ".html", ".tmpl"}
    if any(parent == directory or parent.startswith(directory + "/") for directory in FRONTEND_SOURCE_DIRS):
        return path.suffix.casefold() in FRONTEND_EXTENSIONS
    if parent == "tools":
        return path.suffix.casefold() == ".py"
    if parent == "tests":
        return path.suffix.casefold() == ".py"
    if parent == ".github/workflows":
        return path.suffix.casefold() in {".yml", ".yaml"}
    return False


def is_link_or_junction(path: Path) -> bool:
    return path.is_symlink() or getattr(path, "is_junction", lambda: False)()


def repository_file_allowed(root: Path, path: Path) -> bool:
    """Apply the allowlist without following a linked file or ancestor directory."""
    root = root.resolve()
    try:
        relative = path.relative_to(root).as_posix()
        path.resolve().relative_to(root)
    except ValueError:
        return False
    current = path
    while current != root:
        if is_link_or_junction(current):
            return False
        current = current.parent
    return checkpoint_path_allowed(relative)
