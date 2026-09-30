import shutil
import subprocess
import sys
from pathlib import Path

def main():
    if shutil.which("go") is None:
        print("Error: 'go' is not installed or not in PATH.", file=sys.stderr)
        return

    executable = Path("eid")
    if executable.exists():
        if executable.is_dir():
            shutil.rmtree(executable)
        else:
            executable.unlink()

    subprocess.run(["go", "get", "github.com/charmbracelet/bubbletea"])
    subprocess.run(["go", "mod", "tidy"])
    subprocess.run(["go", "build"])
    subprocess.run(["./eid", sys.argv[1]])

if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        print(f"An error occurred: {e}")