#!/usr/bin/env python3
import os
import sys
import json
import hashlib
from pathlib import Path

def calculate_sha256(file_path):
    """Calculate SHA-256 checksum of a file."""
    sha256_hash = hashlib.sha256()
    try:
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                sha256_hash.update(chunk)
        return sha256_hash.hexdigest()
    except Exception as e:
        return f"ERROR: {str(e)}"

def generate_manifest(stage_dir):
    """Generate manifest.json with file checksums."""
    stage_path = Path(stage_dir)
    if not stage_path.exists():
        print(f"ERROR: Stage directory {stage_dir} does not exist", file=sys.stderr)
        sys.exit(1)
    
    manifest = {
        "timestamp": stage_path.name.split('_', 1)[1] if '_' in stage_path.name else "unknown",
        "total_files": 0,
        "files": {}
    }
    
    # Find all files recursively and sort them
    all_files = []
    for root, dirs, files in os.walk(stage_path):
        for file in files:
            file_path = Path(root) / file
            rel_path = file_path.relative_to(stage_path)
            all_files.append((str(rel_path), str(file_path)))
    
    all_files.sort(key=lambda x: x[0])  # Sort by relative path
    
    # Calculate checksums
    for rel_path, abs_path in all_files:
        checksum = calculate_sha256(abs_path)
        file_size = os.path.getsize(abs_path) if os.path.exists(abs_path) else 0
        
        manifest["files"][rel_path] = {
            "sha256": checksum,
            "size_bytes": file_size
        }
        manifest["total_files"] += 1
    
    # Write manifest
    manifest_path = stage_path / "manifest.json"
    with open(manifest_path, 'w', encoding='utf-8') as f:
        json.dump(manifest, f, indent=2, sort_keys=True)
    
    print(f"Manifest generated: {manifest['total_files']} files")
    return manifest_path

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 generate_manifest.py <stage_directory>", file=sys.stderr)
        sys.exit(1)
    
    generate_manifest(sys.argv[1])