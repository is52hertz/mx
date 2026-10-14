"""Exercise the installer without network access or real package installation."""
import hashlib
import io
import os
from pathlib import Path
import pty
import select
import shutil
import subprocess
import tarfile
import tempfile
import unittest
import fcntl
import termios


SCRIPT = Path(__file__).resolve().parents[1] / "install.sh"
BASH = shutil.which("bash")


class InstallTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.home = self.root / "home"
        self.home.mkdir()
        self.env = dict(os.environ, HOME=str(self.home), PATH=str(self.bin),
                        FIXTURE=str(self.root), TEST_OS="Darwin", TEST_ARCH="arm64")
        for tool in ("tar", "awk", "mktemp", "cp", "chmod", "mv", "rm", "mkdir"):
            (self.bin / tool).symlink_to(shutil.which(tool))
        checksum = "sha256sum" if shutil.which("sha256sum") else "shasum"
        (self.bin / checksum).symlink_to(shutil.which(checksum))
        self.stub("uname", 'if [[ "$1" == -s ]]; then echo "$TEST_OS"; else echo "$TEST_ARCH"; fi')
        payload = b"#!/bin/sh\necho fixture-mx\n"
        with tarfile.open(self.root / "archive.tar.gz", "w:gz") as archive:
            entry = tarfile.TarInfo("mx")
            entry.size = len(payload)
            entry.mode = 0o755
            archive.addfile(entry, io.BytesIO(payload))
        digest = hashlib.sha256((self.root / "archive.tar.gz").read_bytes()).hexdigest()
        (self.root / "SHA256SUMS").write_text("".join(
            f"{digest}  mx_{system}_{arch}.tar.gz\n"
            for system in ("darwin", "linux") for arch in ("amd64", "arm64")))
        self.stub("curl", '''
output=""
url=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        -o) output="$2"; shift 2 ;;
        https://*) url="$1"; shift ;;
        *) shift ;;
    esac
done
case "$url" in
    */releases/latest) printf 'https://github.com/is52hertz/mx/releases/tag/v0.1.0' ;;
    */SHA256SUMS) cp "$FIXTURE/SHA256SUMS" "$output" ;;
    *.tar.gz) [[ "${FAIL_DOWNLOAD:-}" != 1 ]] || exit 22
              cp "$FIXTURE/archive.tar.gz" "$output" ;;
    *) exit 22 ;;
esac
''')

    def stub(self, name, body="exit 0"):
        path = self.bin / name
        path.write_text(f"#!{BASH}\nset -eu\n{body}\n")
        path.chmod(0o755)

    def tools(self):
        for name in ("ffmpeg", "ffprobe", "ncm2mp3"):
            self.stub(name)

    def run_script(self, answers=None):
        if answers is None:
            result = subprocess.run([BASH, str(SCRIPT)], env=self.env,
                                    stdin=subprocess.DEVNULL, capture_output=True,
                                    text=True, start_new_session=True, timeout=10)
            return result.returncode, result.stdout + result.stderr
        master, slave = pty.openpty()

        def terminal():
            os.setsid()
            fcntl.ioctl(slave, termios.TIOCSCTTY, 0)

        # The script's stdin is a pipe, like curl | bash; answers use /dev/tty.
        proc = subprocess.Popen([BASH], env=self.env, stdin=subprocess.PIPE,
                                stdout=slave, stderr=slave, preexec_fn=terminal)
        os.close(slave)
        proc.stdin.write(SCRIPT.read_bytes())
        proc.stdin.close()
        output = b""
        sent = 0
        try:
            while True:
                ready, _, _ = select.select([master], [], [], 10)
                if not ready:
                    raise AssertionError("Installer timed out")
                try:
                    chunk = os.read(master, 65536)
                except OSError:
                    break
                if not chunk:
                    break
                output += chunk
                if output.count(b"[y/N]") > sent:
                    os.write(master, (answers[sent] + "\n").encode())
                    sent += 1
            return proc.wait(timeout=5), output.decode()
        finally:
            os.close(master)
            if proc.poll() is None:
                proc.kill()
                proc.wait()

    def test_existing_dependencies(self):
        self.tools()
        code, output = self.run_script()
        self.assertEqual(code, 0, output)
        self.assertEqual((self.home / ".local/bin/mx").stat().st_mode & 0o777, 0o755)
        self.assertIn('export PATH="$HOME/.local/bin:$PATH"', output)

    def test_missing_cargo_is_optional(self):
        self.stub("ffmpeg")
        self.stub("ffprobe")
        code, output = self.run_script()
        self.assertEqual(code, 0, output)
        self.assertIn("未检测到可用的 Cargo", output)

    def test_cargo_without_toolchain_is_reported(self):
        self.stub("ffmpeg")
        self.stub("ffprobe")
        self.stub("cargo", 'exit 1')
        code, output = self.run_script()
        self.assertEqual(code, 0, output)
        self.assertIn("未检测到可用的 Cargo", output)

    def test_noninteractive_never_installs(self):
        for tool in ("brew", "cargo"):
            self.stub(tool, f'''[[ "${{1:-}}" != --version ]] || exit 0
printf unexpected > "$FIXTURE/unexpected-{tool}"
exit 99''')
        code, output = self.run_script()
        self.assertEqual(code, 1, output)
        self.assertIn("brew install ffmpeg", output)
        self.assertFalse(list(self.root.glob("unexpected-*")))

    def test_decline_interactive_install(self):
        self.stub("brew", 'exit 99')
        self.stub("cargo", '[[ "${1:-}" != --version ]] || exit 0; exit 99')
        code, output = self.run_script(["n", ""])
        self.assertEqual(code, 1, output)
        self.assertEqual(output.count("[y/N]"), 2)

    def test_accept_interactive_install(self):
        self.stub("brew", '''
printf '%s\\n' "$*" > "$FIXTURE/brew-call"
cp "$FIXTURE/bin/brew" "$FIXTURE/bin/ffmpeg"
cp "$FIXTURE/bin/brew" "$FIXTURE/bin/ffprobe"
''')
        self.stub("cargo", 'printf "%s\\n" "$*" > "$FIXTURE/cargo-call"')
        code, output = self.run_script(["y", "yes"])
        self.assertEqual(code, 0, output)
        self.assertEqual((self.root / "brew-call").read_text().strip(), "install ffmpeg")
        self.assertEqual((self.root / "cargo-call").read_text().strip(), "install ncm2mp3 --locked")

    def test_package_failure_reports_missing_dependencies(self):
        self.stub("brew", 'exit 1')
        self.stub("ncm2mp3")
        code, output = self.run_script(["y"])
        self.assertEqual(code, 1, output)
        self.assertIn("FFmpeg 安装失败", output)

    def test_linux_apt_command(self):
        self.env.update(TEST_OS="Linux", TEST_ARCH="x86_64")
        self.stub("apt-get")
        self.stub("sudo")
        self.stub("ncm2mp3")
        code, output = self.run_script()
        self.assertEqual(code, 1, output)
        self.assertIn("apt-get install -y ffmpeg", output)

    def test_checksum_failure_preserves_existing_binary(self):
        target = self.home / ".local/bin/mx"
        target.parent.mkdir(parents=True)
        target.write_text("old binary")
        (self.root / "SHA256SUMS").write_text("0" * 64 + "  mx_darwin_arm64.tar.gz\n")
        code, output = self.run_script()
        self.assertNotEqual(code, 0, output)
        self.assertIn("SHA256 校验失败", output)
        self.assertEqual(target.read_text(), "old binary")

    def test_download_failure_does_not_install(self):
        self.env["FAIL_DOWNLOAD"] = "1"
        code, output = self.run_script()
        self.assertNotEqual(code, 0, output)
        self.assertFalse((self.home / ".local/bin/mx").exists())

    def test_unsupported_system(self):
        self.env["TEST_OS"] = "Windows_NT"
        code, output = self.run_script()
        self.assertNotEqual(code, 0, output)
        self.assertIn("目前仅支持 macOS/Linux", output)


if __name__ == "__main__":
    unittest.main()
