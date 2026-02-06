import click
import sys
from pathlib import Path
from .installer import GameInstaller

@click.group()
def cli():
    """Deck Game Installer - Automate Windows game installation on Steam Deck."""
    pass

@cli.command()
@click.argument('file_path', type=click.Path(exists=True, path_type=Path))
def install(file_path: Path):
    """Install a game from an ISO or EXE file."""
    installer = GameInstaller()
    installer.install(file_path)

def main():
    try:
        cli()
    except Exception as e:
        # Final safety net
        from .kdialog import KDialog
        KDialog.error("Fatal Error", str(e))
        sys.exit(1)

if __name__ == "__main__":
    main()
