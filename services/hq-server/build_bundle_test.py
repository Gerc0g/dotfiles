"""Release packer contract tests; fixtures never use host credentials or builds."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


spec = importlib.util.spec_from_file_location("hq_bundle", Path(__file__).with_name("build-bundle.py"))
packer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packer)


class BundleTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for relative in ("hq", "relay/happier-server", "cli/happier", "codex/bin/codex", "ui/index.html", "skills/research/SKILL.md", "templates/AGENTS.md.company.tmpl", "templates/AGENTS.md.product.tmpl", "templates/AGENTS.md.repo.tmpl", "relay/node_modules/@prisma/client/package.json", "relay/node_modules/.prisma/client/index.js", "relay/node_modules/.prisma/client/libquery_engine-debian-openssl-3.0.x.so.node", "relay/prisma/sqlite/migrations/migration_lock.toml", "relay/prisma/sqlite/migrations/0001/migration.sql"):
            item = self.root / relative
            item.parent.mkdir(parents=True, exist_ok=True)
            item.write_text(relative)
            item.chmod(0o755 if relative in ("hq", "relay/happier-server", "cli/happier", "codex/bin/codex") else 0o644)
        self.args = argparse.Namespace(**{key: str(self.root / key) for key in ("hq", "cli", "relay", "ui", "skills", "templates", "codex")}, version="v1", platform="linux-amd64", output=str(self.root / "bundle"), codex_image=None)

    def test_materializes_internal_link_and_hashes_every_file(self):
        (self.root / "cli/alias").symlink_to("happier")
        output = packer.build(self.args)
        manifest = json.loads((output / "manifest.json").read_text())
        self.assertFalse((output / "cli/alias").is_symlink())
        self.assertEqual((output / "cli/alias").read_text(), "cli/happier")
        for relative, item in manifest["files"].items():
            self.assertEqual(hashlib.sha256((output / relative).read_bytes()).hexdigest(), item["sha256"])
        self.assertEqual(manifest["files"]["hq"]["mode"], 0o755)

    def test_escaping_link_and_credentials_leave_no_bundle(self):
        for forbidden in ("escape", "auth.json"):
            with self.subTest(forbidden=forbidden):
                item = self.root / "cli" / forbidden
                if forbidden == "escape":
                    item.symlink_to("../hq")
                else:
                    item.write_text("fixture")
                with self.assertRaises(ValueError):
                    packer.build(self.args)
                item.unlink()
                self.assertFalse(Path(self.args.output).exists())
                self.assertEqual(list(self.root.glob(".hq-bundle-*")), [])

    def test_missing_skills_is_rejected(self):
        (self.root / "skills/research/SKILL.md").unlink()
        with self.assertRaisesRegex(ValueError, "skills"):
            packer.build(self.args)

    def test_missing_onboarding_template_is_rejected(self):
        (self.root / "templates/AGENTS.md.repo.tmpl").unlink()
        with self.assertRaisesRegex(ValueError, "template"):
            packer.build(self.args)

    def test_missing_relay_engine_is_rejected(self):
        (self.root / "relay/node_modules/.prisma/client/libquery_engine-debian-openssl-3.0.x.so.node").unlink()
        with self.assertRaisesRegex(ValueError, "Relay runtime closure"):
            packer.build(self.args)
        self.assertFalse(Path(self.args.output).exists())


if __name__ == "__main__":
    unittest.main()
