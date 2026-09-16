"""Create local source deltas without overwriting checkpoints or touching Git credentials."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import uuid
import zipfile

from checkpoint_policy import SENSITIVE_PARTS, is_link_or_junction, repository_file_allowed


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_stream(stream) -> str:
    digest = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        digest.update(chunk)
    return digest.hexdigest()


def iter_repository_files(root: Path):
    """Walk only non-linked, non-runtime directories; mutate dirs to prevent descent."""
    for current, dirs, filenames in os.walk(root, topdown=True, followlinks=False):
        current_path = Path(current)
        dirs[:] = [
            name for name in dirs
            if name.casefold() not in SENSITIVE_PARTS
            and not is_link_or_junction(current_path / name)
        ]
        if is_link_or_junction(current_path):
            dirs[:] = []
            continue
        for filename in filenames:
            yield current_path / filename


def publish_new(temp: Path, destination: Path) -> None:
    """Publish a complete temporary file without replacing an existing checkpoint."""
    os.link(temp, destination)
    temp.unlink()


def create_checkpoint(root: Path, output_dir: Path, label: str) -> dict:
    root = root.resolve()
    output_dir = output_dir.resolve()
    if not output_dir.is_relative_to(root):
        raise ValueError("checkpoint output outside repository")
    if not label.replace("-", "").isalnum():
        raise ValueError("invalid checkpoint label")
    base = json.loads((output_dir / "baseline.json").read_text(encoding="utf-8"))
    zip_path = output_dir / f"{label}.zip"
    json_path = output_dir / f"{label}.json"
    if zip_path.exists() or json_path.exists():
        raise FileExistsError("checkpoint output already exists")

    files = set()
    for name in base.get("sha256", {}):
        if repository_file_allowed(root, root / name):
            files.add(name)
    for candidate in iter_repository_files(root):
        if candidate.is_file() and repository_file_allowed(root, candidate):
            files.add(candidate.relative_to(root).as_posix())

    changed, missing = {}, []
    for name in sorted(files):
        path = root / name
        if not path.is_file():
            if name in base.get("sha256", {}):
                missing.append(name)
            continue
        digest = sha256(path)
        if digest != base.get("sha256", {}).get(name):
            changed[name] = digest
    index_path = root / ".git" / "index"
    manifest = {
        "label": label,
        "head_baseline": base["head"],
        "changed_from_baseline": changed,
        "missing_from_baseline": missing,
        "git_index_unchanged": index_path.is_file() and sha256(index_path) == base.get("index_sha256"),
        "restore_scope": "delta only; not a complete Git recovery",
    }

    temporary_zip = output_dir / f".{label}.{uuid.uuid4().hex}.zip.tmp"
    temporary_json = output_dir / f".{label}.{uuid.uuid4().hex}.json.tmp"
    with zipfile.ZipFile(temporary_zip, "x", zipfile.ZIP_DEFLATED) as archive:
        for name in changed:
            archive.write(root / name, name)
        archive.writestr("checkpoint-manifest.json", json.dumps(manifest, ensure_ascii=False, indent=2))
    with zipfile.ZipFile(temporary_zip) as archive:
        if archive.testzip() is not None:
            raise ValueError("checkpoint ZIP integrity check failed")
        for name, digest in changed.items():
            with archive.open(name) as stream:
                if sha256_stream(stream) != digest:
                    raise ValueError("checkpoint ZIP source hash mismatch")
    temporary_json.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    # If publication fails, the single named temporary file remains for diagnosis.
    publish_new(temporary_zip, zip_path)
    publish_new(temporary_json, json_path)
    return manifest


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("label")
    parser.add_argument("--repo", default=Path(__file__).resolve().parents[1])
    parser.add_argument("--output-dir")
    args = parser.parse_args(argv)
    root = Path(args.repo)
    output_dir = Path(args.output_dir) if args.output_dir else root / ".local" / "autonomous-backlog"
    manifest = create_checkpoint(root, output_dir, args.label)
    print("Checkpoint verified:", args.label, len(manifest["changed_from_baseline"]), "files; missing:", len(manifest["missing_from_baseline"]), "; original index unchanged:", manifest["git_index_unchanged"])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
