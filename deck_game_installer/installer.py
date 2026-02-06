import time
import os
import subprocess
from pathlib import Path
from typing import Optional, List, Tuple

import shutil
import tempfile
from .kdialog import KDialog, Logger, LogWindow
from .iso import ISOManager
from .steam import SteamManager
from .proton import ProtonManager

class GameInstaller:
    def __init__(self):
        self.steam = SteamManager()
        self.proton = ProtonManager()
        self.iso_manager = ISOManager()
        self.logger = Logger()
        self.log_win = LogWindow("Deck Game Installer")
        
        # Link logger to log window for real-time updates
        self.logger.on_log = self.log_win.write

    def install(self, file_path: Path):
        self.log_win.open()
        self.logger.log(f"--- STEP 1: INITIALIZING ---")
        self.logger.log(f"Starting installation for: {file_path}")
        try:
            if file_path.suffix.lower() == ".iso":
                self._install_from_iso(file_path)
            elif file_path.suffix.lower() == ".exe":
                self._install_from_exe(file_path)
            else:
                self.logger.log(f"Unsupported file type: {file_path.suffix}")
                KDialog.error("Error", f"Unsupported file type: {file_path.suffix}. Supported: .iso, .exe")
        except Exception as e:
            self.logger.log(f"Error during installation: {str(e)}")
            KDialog.error("Installation Error", f"See log window for details: {str(e)}")
        finally:
            self.iso_manager.unmount()
            self.logger.log("\n--- INSTALLATION FINISHED ---")
            self.logger.log("You can close this window now.")
            self.log_win.wait()

    def _install_from_iso(self, iso_path: Path):
        mount_point = None
        smb_mount_point = None
        original_iso_path = iso_path
        temp_iso = None
        allow_sudo = False
        
        # --- Check for Network Share (kio-fuse) ---
        from .iso import SMBRemounter
        smb_info = SMBRemounter.parse_kio_path(iso_path)
        
        if smb_info and not allow_sudo:
            self.logger.log("\n--- DETECTED NETWORK SHARE ---")
            self.logger.log(f"Server: {smb_info['server']}, Share: {smb_info['share']}")
            self.logger.log("Automatically remounting SMB share as system drive (requires admin password)...")
            allow_sudo = True

        if smb_info and allow_sudo:
            self.logger.log("\n--- DETECTED NETWORK SHARE ---")
            self.logger.log(f"Server: {smb_info['server']}, Share: {smb_info['share']}")
            self.logger.log("Remounting share as System Drive to bypass FUSE limits...")
            
            try:
                # Try guest mount first
                smb_mount_point = SMBRemounter.mount_cifs(smb_info['server'], smb_info['share'])
                self.logger.log(f"SMB Share remounted at: {smb_mount_point}")
                
                # Update iso_path to point to the new location
                iso_path = smb_mount_point / smb_info['rel_path']
                self.logger.log(f"New ISO Path: {iso_path}")
                
            except Exception as smb_err:
                self.logger.log(f"Failed to remount SMB share: {smb_err}")
                # Fall through to standard mount (which will likely fail, then fallback to copy)

        # --- Standard Mount (udisksctl) ---
        # Now that we typically have a local path or kernel mount, this should work.
        try:
            self.logger.log(f"\n--- STEP 2: MOUNTING ISO ---")
            self.logger.log(f"Mounting: {iso_path}")
            mount_point = self.iso_manager.mount(iso_path)
        except Exception as mount_err:
            self.logger.log(f"Standard mount failed: {mount_err}")
            
            # If standard mount failed on the remounted path, try root mount (only when allowed)
            # (Root mount might work on CIFS if udisksctl doesn't like it)
            if not mount_point and allow_sudo:
                try:
                    self.logger.log("Attempting Root Mount...")
                    mount_point = self.iso_manager.mount_root(iso_path)
                except Exception as root_err:
                    self.logger.log(f"Root mount failed: {root_err}")

        # --- Fallback to Local Copy ---
        if not mount_point:
            # If standard mount fails (likely network share), offer to try as Admin (Root)
            # This avoids copying the file if possible.
            if KDialog.question(
                "Standard Mount Failed",
                "Unable to mount the ISO normally (likely due to network restrictions).\n\n"
                "Attempt to mount as Administrator (Root)?\n"
                "(You will be prompted for your password)",
            ):
                self.logger.log("Attempting to mount as Root (pkexec)...")
                try:
                    mount_point = self.iso_manager.mount_root(iso_path)
                    self.logger.log(f"Root mount successful at: {mount_point}")
                except Exception as root_err:
                    self.logger.log(f"Root mount failed: {root_err}")
                    mount_point = None
            else:
                mount_point = None

            # --- Fallback to Local Copy ---
            if not mount_point:
                self.logger.log("Root mount failed or declined.")
                if KDialog.question(
                    "Copy ISO Locally?",
                    "All mount attempts failed.\n\n"
                    "Would you like to copy the ISO to a local temporary directory and try again?\n\n"
                    f"File: {iso_path.name}\n"
                    "Note: This may take a while and requires enough free disk space.",
                ):
                    self.logger.log("Copying ISO to local temporary directory...")
                    try:
                        # Create a temp dir that will be cleaned up on logout/reboot,
                        # but we'll also try to clean it up manually.
                        temp_dir = Path(tempfile.gettempdir()) / "steamer_iso_cache"
                        temp_dir.mkdir(parents=True, exist_ok=True)
                        temp_iso = temp_dir / iso_path.name

                        self.logger.log(f"Destination: {temp_iso}")
                        shutil.copy2(iso_path, temp_iso)
                        self.logger.log("Copy complete. Attempting to mount local copy...")

                        mount_point = self.iso_manager.mount(temp_iso)
                    except Exception as copy_err:
                        self.logger.log(f"Failed to copy/mount local ISO: {copy_err}")
                        if temp_iso and temp_iso.exists():
                            temp_iso.unlink()
                        raise
                else:
                    self.logger.log("User declined local copy. Aborting.")
                    return

        if not mount_point:
            if KDialog.question(
                "Copy ISO Locally?",
                "All mount attempts failed.\n\n"
                "Would you like to copy the ISO to a local temporary directory and try again?\n\n"
                f"File: {iso_path.name}\n"
                "Note: This may take a while and requires enough free disk space.",
            ):
                self.logger.log("Copying ISO to local temporary directory...")
                try:
                    temp_dir = Path(tempfile.gettempdir()) / "steamer_iso_cache"
                    temp_dir.mkdir(parents=True, exist_ok=True)
                    temp_iso = temp_dir / iso_path.name

                    self.logger.log(f"Destination: {temp_iso}")
                    shutil.copy2(iso_path, temp_iso)
                    self.logger.log("Copy complete. Attempting to mount local copy...")

                    mount_point = self.iso_manager.mount(temp_iso)
                except Exception as copy_err:
                    self.logger.log(f"Failed to copy/mount local ISO: {copy_err}")
                    if temp_iso and temp_iso.exists():
                        temp_iso.unlink()
                    raise
            else:
                self.logger.log("User declined local copy. Aborting.")
                return

        try:
            self.logger.log(f"Mounted at: {mount_point}")
        
            # Try to get game name from volume label (mount point name), falling back to ISO filename
            volume_label = mount_point.name
            if volume_label.startswith("steamer_mnt_") or len(volume_label) <= 3:
                game_name = iso_path.stem
            else:
                game_name = volume_label
            
            self.logger.log(f"Detected game name: {game_name}")
            
            # Check if mount works
            try:
                files = list(mount_point.iterdir())
                if not files:
                    self.logger.log("Warning: Mount point appears empty!")
                    self.logger.log("archivemount may not support this ISO format (e.g. UDF).")
            except Exception as e:
                self.logger.log(f"Warning: Failed to list mount point: {e}")

            # Close log window briefly to show selection with context
            self.log_win.close()
            installer_path = self._select_installer_from_mount(mount_point)
            self.log_win.open()
            
            if installer_path:
                self.logger.log(f"Selected installer: {installer_path}")
                self._run_installation_workflow(installer_path, game_name)
            else:
                self.logger.log("No installer selected, cancelling.")
        finally:
            if temp_iso and temp_iso.exists():
                self.logger.log(f"Cleaning up temporary ISO: {temp_iso.name}")
                try:
                    temp_iso.unlink()
                except Exception as cleanup_err:
                    self.logger.log(f"Warning: Could not delete temp ISO: {cleanup_err}")
            
            # Cleanup SMB Remount
            if smb_mount_point:
                self.logger.log(f"Unmounting temporary SMB share...")
                from .iso import SMBRemounter
                SMBRemounter.unmount(smb_mount_point)

    def _install_from_exe(self, exe_path: Path):
        self._run_installation_workflow(exe_path, exe_path.stem)

    def _select_installer_from_mount(self, mount_point: Path) -> Optional[Path]:
        installers = self.iso_manager.find_installer(mount_point)
        self.logger.log(f"Found {len(installers)} executables in mount.")
        
        options = [(str(p), p.name) for p in installers]
        options.append(("__browse__", "Browse for another executable..."))
        
        recent_logs = self.logger.get_recent(5)
        msg = f"LOGS:\n{recent_logs}\n\nSelect the installer executable:"
        
        selected = KDialog.radio_list("Select Installer", options, message=msg, default=options[0][0] if options else None)
        
        if selected == "__browse__":
            return KDialog.select_file("Select Installer", mount_point, "*.exe")
        return Path(selected) if selected else None

    def _run_installation_workflow(self, installer_path: Path, game_name: str):
        # 1. Add installer to Steam
        self.logger.log(f"\n--- STEP 3: ADDING TO STEAM ---")
        
        # Clean up the game name (remove underscores, handle common patterns)
        clean_game_name = self._clean_game_name(game_name)
        installer_shortcut_name = f"{clean_game_name} (Installer)"
        
        # Check if already exists first
        app_id_32 = self.steam.find_app_id_by_path(str(installer_path))
        if app_id_32:
            self.logger.log(f"Shortcut for installer already exists (AppID: {app_id_32}).")
            self.logger.log("Reusing existing shortcut.")
        else:
            self.logger.log(f"Adding '{installer_shortcut_name}' to Steam library...")
            try:
                app_id_32 = self.steam.add_shortcut(installer_shortcut_name, str(installer_path))
                self.logger.log(f"Successfully added to Steam (AppID: {app_id_32}).")
            except Exception as e:
                self.logger.log(f"Failed to add shortcut: {e}")
                self.logger.log("Attempting to proceed anyway...")
                app_id_32 = self.steam.generate_app_id(str(installer_path), installer_shortcut_name)
            
        if not app_id_32:
            app_id_32 = self.steam.generate_app_id(str(installer_path), installer_shortcut_name)
            self.logger.log(f"Note: Using predicted ID: {app_id_32}")

        # --- Automatically set Proton for the Installer ---
        proton_versions = self.proton.get_available_proton_versions()
        experimental = next((v for v in proton_versions if "experimental" in v.lower()), None)
        default_proton = experimental or (proton_versions[0] if proton_versions else None)
        
        # Check if already configured
        existing_proton = self.steam.get_proton_version(app_id_32)
        needs_restart = False
        
        if existing_proton:
            self.logger.log(f"Proton already configured: {existing_proton}")
        elif default_proton:
            self.logger.log(f"Assigning {default_proton} to installer...")
            try:
                self.steam.set_proton_version(app_id_32, default_proton)
                self.logger.log("Installer Proton configuration applied.")
                needs_restart = True
            except Exception as e:
                self.logger.log(f"Warning: Could not set Proton for installer: {e}")
        
        # Only prompt for restart if we made changes
        if needs_restart:
            self.log_win.close()
            self.log_win.open(ok_label="Restart Now", cancel_label="Later")
            self.logger.log("\n>>> STEAM RESTART REQUIRED <<<")
            self.logger.log("Steam must be restarted to recognize the new shortcut and Proton settings.")
            
            if self.log_win.wait():
                self.logger.log("Restarting Steam...")
                self.steam.restart_steam()
                self.logger.log("Waiting for Steam to restart...")
                time.sleep(10)
            else:
                self.logger.log("User chose to restart later.")
            
            self.log_win.open()
        
        app_id_url = self.steam.get_url_app_id_from_u32(app_id_32)

        # 3. Launch via Steam
        self.logger.log(f"\n--- STEP 4: RUNNING INSTALLER ---")
        self.logger.log(f"Launching installer (AppID: {app_id_url})...")
        subprocess.Popen(["steam", f"steam://rungameid/{app_id_url}"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        
        self.logger.log("\n>>> ACTION REQUIRED <<<")
        self.logger.log("1. Complete the installation in the window that opened.")
        self.logger.log("2. Once the installation is finished, CLICK 'OK' BELOW to continue.")
        
        if not self.log_win.wait():
            self.logger.log("User cancelled.")
            return

        self.log_win.open()

        # 4. Scan prefix for game executables
        self.logger.log(f"\n--- STEP 5: FINDING GAME ---")
        self.logger.log("Scanning Proton prefix for game executables...")
        executables = self.proton.scan_prefix_for_executables(app_id_32)
        if not executables:
            self.logger.log("No executables found in prefix.")
            KDialog.error("Error", "No game executables found. Installation may have failed.")
            return

        # 5. User selects game exe and Proton version
        self.log_win.close()
        selected_exe, selected_proton = self._select_game_and_proton(executables)
        self.log_win.open()
        
        if selected_exe:
            self.logger.log(f"Selected game: {selected_exe}")
            if selected_proton:
                self.logger.log(f"Selected Proton: {selected_proton}")
            
            # 6. Finalizing
            self.logger.log(f"\n--- STEP 6: FINALIZING ---")
            
            # Check if final game already exists
            final_app_id = self.steam.find_app_id_by_path(str(selected_exe))
            if final_app_id:
                self.logger.log(f"Final game shortcut already exists (AppID: {final_app_id}).")
            else:
                self.logger.log(f"Adding '{game_name}' to Steam library...")
                try:
                    final_app_id = self.steam.add_shortcut(game_name, str(selected_exe))
                    self.logger.log(f"Added '{game_name}' with AppID: {final_app_id}")
                except Exception as e:
                    self.logger.log(f"Failed to add final game: {e}")
                    KDialog.error("Error", f"Failed to finalize: {e}")
                    return

            if final_app_id and selected_proton:
                self.logger.log(f"Assigning {selected_proton} to game...")
                self.steam.set_proton_version(final_app_id, selected_proton)
                self.logger.log("Proton version assigned successfully.")
            
            self.logger.log("\nSuccessfully completed installation!")
            
            if KDialog.question("Restart Steam?", "Proton settings require a Steam restart to take effect.\n\nWould you like to restart Steam now?"):
                self.logger.log("Restarting Steam...")
                self.steam.restart_steam()
            else:
                self.logger.log("Please restart Steam manually to apply Proton settings.")

    def _select_game_and_proton(self, executables: List[Tuple[str, Path]]) -> Tuple[Optional[Path], Optional[str]]:
        exe_options = [(str(p), name) for name, p in executables]
        recent_logs = self.logger.get_recent(5)
        
        selected_exe_str = KDialog.radio_list("Select Game", exe_options, message=f"{recent_logs}\n\nSelect the game executable:", default=exe_options[0][0])
        if not selected_exe_str:
            return None, None
            
        proton_versions = self.proton.get_available_proton_versions()
        experimental = next((v for v in proton_versions if "experimental" in v.lower()), None)
        
        selected_proton = KDialog.radio_list("Select Proton", [(v, v) for v in proton_versions], 
                                            message=f"Select Proton version for '{Path(selected_exe_str).name}':", 
                                            default=experimental or (proton_versions[0] if proton_versions else None))
        
        return Path(selected_exe_str), selected_proton

    @staticmethod
    def _clean_game_name(raw_name: str) -> str:
        """Cleans up a raw game name (e.g., from ISO filename) into a human-readable format.
        
        Examples:
            'Doom_Eternal_v1.0' -> 'Doom Eternal'
            'The.Witcher.3-GOG' -> 'The Witcher 3'
            'ELDEN_RING_DELUXE' -> 'ELDEN RING DELUXE'
        """
        import re
        name = raw_name
        
        # Remove common suffixes
        patterns_to_remove = [
            r'[_\-\.]?(v\d+[\d\.]*)',  # Version numbers (v1.0, _v2.3.1)
            r'[_\-\.]?(GOG|CODEX|PLAZA|SKIDROW|RELOADED|FitGirl|DODI)$',  # Scene groups
            r'[_\-\.]?(x64|x86|64bit|32bit)$',  # Architecture
            r'[_\-\.]?(Setup|Install|Installer)$',  # Installer suffixes
        ]
        for pattern in patterns_to_remove:
            name = re.sub(pattern, '', name, flags=re.IGNORECASE)
        
        # Replace underscores and dots with spaces
        name = name.replace('_', ' ').replace('.', ' ')
        
        # Clean up multiple spaces and trim
        name = re.sub(r'\s+', ' ', name).strip()
        
        # Remove trailing dashes
        name = name.rstrip('- ')
        
        return name or raw_name  # Fallback to original if we stripped everything
