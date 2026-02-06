import subprocess
import re
from pathlib import Path
from typing import List, Optional

class ISOManager:
    def __init__(self):
        self.mount_point: Optional[Path] = None
        self.loop_device: Optional[str] = None
        self.is_root_mount: bool = False

    def mount(self, iso_path: Path) -> Path:
        existing = self._find_existing_mount(iso_path)
        if existing:
            self.loop_device, self.mount_point = existing
            self.is_root_mount = False
            return self.mount_point
        try:
            # 1. Setup loop device: udisksctl loop-setup -f <iso_path>
            setup_result = subprocess.run(
                ["udisksctl", "loop-setup", "-f", str(iso_path)],
                capture_output=True, text=True, check=True
            )
            # Output: Mapped file /path/to/file.iso as /dev/loopN.
            match = re.search(r"as (/dev/loop\d+)\.", setup_result.stdout)
            if not match:
                raise RuntimeError(f"Failed to parse loop device from: {setup_result.stdout}")
            self.loop_device = match.group(1)

            # 2. Mount: udisksctl mount -b <loop_device>
            mount_result = subprocess.run(
                ["udisksctl", "mount", "-b", self.loop_device],
                capture_output=True, text=True, check=True
            )
            # Output: Mounted /dev/loopN at /run/media/user/mountpoint.
            match = re.search(r"at (/\S+)", mount_result.stdout)
            if not match:
                raise RuntimeError(f"Failed to parse mount point from: {mount_result.stdout}")
            self.mount_point = Path(match.group(1).rstrip('.'))
            self.is_root_mount = False
            
            return self.mount_point
        except subprocess.CalledProcessError as e:
            details = e.stderr.strip() if e.stderr else (e.stdout.strip() if e.stdout else str(e))
            raise RuntimeError(f"Failed to mount ISO: {details}") from e

    def _find_existing_mount(self, iso_path: Path) -> Optional[tuple[str, Path]]:
        """Check if ISO is already attached to a loop device and mounted."""
        try:
            # losetup -j <iso> returns lines like: /dev/loop0: [...] (/path/to.iso)
            result = subprocess.run(["losetup", "-j", str(iso_path)], capture_output=True, text=True)
            if result.returncode != 0 or not result.stdout.strip():
                return None
            loop_device = result.stdout.split(":", 1)[0].strip()
            if not loop_device:
                return None

            # Find mountpoint, if any
            mp_result = subprocess.run(["lsblk", "-no", "MOUNTPOINT", loop_device], capture_output=True, text=True)
            mount_point = mp_result.stdout.strip()
            if mount_point:
                return loop_device, Path(mount_point)

            # If attached but not mounted, mount it
            mount_result = subprocess.run(
                ["udisksctl", "mount", "-b", loop_device],
                capture_output=True, text=True
            )
            if mount_result.returncode == 0:
                match = re.search(r"at (/\S+)", mount_result.stdout)
                if match:
                    return loop_device, Path(match.group(1).rstrip('.'))
        except Exception:
            return None
        return None
    def mount_root(self, iso_path: Path) -> Path:
        """Attempts to mount the ISO using pkexec mount -o loop (requires admin password)."""
        import tempfile
        
        # Create a temp mount point
        mnt_dir = Path(tempfile.mkdtemp(prefix="steamer_mnt_"))
        
        try:
            # pkexec mount -o loop,ro <iso> <mnt_dir>
            subprocess.run(
                ["pkexec", "mount", "-o", "loop,ro", str(iso_path), str(mnt_dir)],
                check=True
            )
            self.mount_point = mnt_dir
            self.is_root_mount = True
            return self.mount_point
        except subprocess.CalledProcessError as e:
            # Cleanup dir if failed
            try:
                mnt_dir.rmdir()
            except:
                pass
            raise RuntimeError(f"Root mount failed: {e}")

    def unmount(self):
        if self.is_root_mount and self.mount_point:
            # Root unmount
            subprocess.run(["pkexec", "umount", str(self.mount_point)], check=False)
            try:
                self.mount_point.rmdir()
            except:
                pass
            self.mount_point = None
            self.is_root_mount = False
            return

        if self.mount_point:
            subprocess.run(["udisksctl", "unmount", "-b", self.loop_device], check=False)
            self.mount_point = None
        
        if self.loop_device:
            subprocess.run(["udisksctl", "loop-delete", "-b", self.loop_device], check=False)
            self.loop_device = None

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.unmount()

    @staticmethod
    def find_installer(search_path: Path) -> List[Path]:
        executables = list(search_path.rglob("*.exe"))
        
        priority_names = ["setup.exe", "install.exe", "installer.exe", "autorun.exe"]
        prioritized = []
        middle = []
        others = []

        for exe in executables:
            name_lower = exe.name.lower()
            if name_lower in priority_names:
                prioritized.append(exe)
            elif any(p in name_lower for p in ["setup", "install", "autorun"]):
                middle.append(exe)
            else:
                others.append(exe)

        # Sort within groups for consistency
        prioritized.sort(key=lambda x: priority_names.index(x.name.lower()) if x.name.lower() in priority_names else 99)
        middle.sort()
        others.sort()

        return prioritized + middle + others


class SMBRemounter:
    """Helper to detect kio-fuse SMB paths and remount them as kernel CIFS mounts."""
    
    @staticmethod
    def parse_kio_path(path: Path) -> Optional[dict]:
        """
        Parses a path like:
        /run/user/1000/kio-fuse-XXXXXX/smb/SERVER/SHARE/Path/To/File.iso
        Returns dict with {server, share, rel_path} or None.
        """
        path_str = str(path)
        # RegEx to capture: .../smb/SERVER/SHARE/(rest)
        match = re.search(r"/smb/([^/]+)/([^/]+)/(.*)$", path_str)
        if match:
            return {
                "server": match.group(1),
                "share": match.group(2),
                "rel_path": match.group(3)
            }
        return None

    @staticmethod
    def mount_cifs(server: str, share: str, username: str = "guest", password: str = "") -> Path:
        """
        Mounts //server/share to a temp directory using pkexec mount -t cifs.
        Returns the mount point.
        """
        import tempfile
        mnt_dir = Path(tempfile.mkdtemp(prefix="steamer_smb_"))
        
        options = ["ro"] # Read-only
        if username == "guest":
            options.append("guest")
        else:
            options.append(f"username={username}")
            if password:
                options.append(f"password={password}")
        
        options_str = ",".join(options)
        share_unc = f"//{server}/{share}"
        
        try:
            subprocess.run(
                ["pkexec", "mount", "-t", "cifs", share_unc, str(mnt_dir), "-o", options_str],
                check=True
            )
            return mnt_dir
        except subprocess.CalledProcessError as e:
            try:
                mnt_dir.rmdir()
            except:
                pass
            raise RuntimeError(f"CIFS mount failed: {e}")

    @staticmethod
    def unmount(mount_point: Path):
        if mount_point and mount_point.exists():
            subprocess.run(["pkexec", "umount", str(mount_point)], check=False)
            try:
                mount_point.rmdir()
            except:
                pass
