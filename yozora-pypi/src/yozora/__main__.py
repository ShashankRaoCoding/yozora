"""Launcher that runs the bundled yzr binary."""
import os
import stat
import subprocess
import sys
from pathlib import Path

NAME = "yzr"


def _binary_path():
    bin_dir = Path(__file__).parent / "bin"
    for candidate in (NAME, NAME + ".exe"):
        path = bin_dir / candidate
        if path.exists():
            return path
    sys.exit(f"{NAME}: bundled binary not found in {bin_dir}")


def main():
    exe = _binary_path()
    if os.name == "nt":
        sys.exit(subprocess.call([str(exe), *sys.argv[1:]]))
    # pip may strip the executable bit; restore it before exec.
    exe.chmod(exe.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    os.execv(exe, [str(exe), *sys.argv[1:]])


if __name__ == "__main__":
    main()
