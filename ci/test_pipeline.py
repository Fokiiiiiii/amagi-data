#!/usr/bin/env python3
"""Offline regression tests for the actual CI planner, preflight and verifier.

Requires Python 3, Bash, Git, jq and Node.js. Git repositories are temporary local
fixtures; no network or credentials are used.
"""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[1]
GENERATOR_PATHS = [
    "main.go", "dataconv", "azurlanelua", "ci",
    ".github/workflows/validate-and-update.yml", "go.mod", "go.sum",
]
REGIONS = ("CN", "EN", "JP", "KR", "TW")
ALIASES = {
    "battlenodescfg": "battle_nodes_cfg",
    "dorm3d_dolly": "dorm3_d_dolly",
    "informcfg": "inform_cfg",
    "informforbackyardthemetemplatecfg": "inform_for_back_yard_theme_template_cfg",
    "world_slgbuff_data": "world_sl_gbuff_data",
}


def run(args: list[str], cwd: Path, *, env: dict[str, str] | None = None,
        check: bool = True, input: str | None = None) -> subprocess.CompletedProcess[str]:
    clean_env = dict(os.environ, GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
                     GIT_TERMINAL_PROMPT="0")
    # Do not inherit repository selection or object-store settings from a caller.
    for key in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY",
                "GIT_ALTERNATE_OBJECT_DIRECTORIES"):
        clean_env.pop(key, None)
    clean_env.update(env or {})
    result = subprocess.run(args, cwd=cwd, env=clean_env, input=input,
                            text=True, capture_output=True, timeout=30)
    if check and result.returncode:
        raise AssertionError(f"{args}: exit {result.returncode}\n{result.stdout}\n{result.stderr}")
    return result


def put(root: Path, rel: str, content: str = "{}\n") -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def init_repo(root: Path) -> None:
    root.mkdir(parents=True)
    run(["git", "init", "-q"], root)
    run(["git", "config", "user.name", "CI fixture"], root)
    run(["git", "config", "user.email", "ci@example.invalid"], root)


def commit(root: Path) -> str:
    run(["git", "add", "--all"], root)
    run(["git", "commit", "--quiet", "--allow-empty", "-m", "test fixture"], root)
    return run(["git", "rev-parse", "HEAD"], root).stdout.strip()


def fingerprint(root: Path) -> str:
    data = run(["git", "ls-tree", "-r", "--full-tree", "HEAD", "--", *GENERATOR_PATHS], root).stdout
    return hashlib.sha256(data.encode()).hexdigest()


class FixtureTest(unittest.TestCase):
    def setUp(self) -> None:
        temp = tempfile.TemporaryDirectory(prefix="amagi-ci-test-")
        self.addCleanup(temp.cleanup)
        self.base = Path(temp.name)
        self.workspace = self.base / "workspace"
        init_repo(self.workspace)
        put(self.workspace, "go.mod", "module fixture\n\ngo 1.25.5\n")
        put(self.workspace, "ci/test.sh", "#!/bin/sh\nexit 0\n")
        (self.workspace / "ci/test.sh").chmod(0o755)
        put(self.workspace, "dataconv/sample.go", "package dataconv\n")
        for n in range(8):
            put(self.workspace, f"dataconv/file_{n}.go", f"package dataconv\n// fixture {n}\n")
        put(self.workspace, "JP/ShareCfg/ignored.json")
        commit(self.workspace)


class WorkflowTests(unittest.TestCase):
    def test_versions_have_one_publishing_workflow(self) -> None:
        planner = (ROOT / "ci/sync-upstream.sh").read_text()
        self.assertIn("global/versions.json", planner)
        self.assertFalse((ROOT / ".github/workflows/sync-versions.yml").exists())


class PreflightTests(FixtureTest):
    def preflight(self, *, latest: str = "a" * 40, old: str = "a" * 40,
                  old_hash: str | None = None, force: bool = False,
                  truncated: bool = False, state_missing: bool = False) -> dict[str, str]:
        workflow = (ROOT / ".github/workflows/validate-and-update.yml").read_text()
        block = workflow.split("          script: |\n", 1)[1].split("\n  build:", 1)[0]
        script = textwrap.dedent(block)
        entries = []
        for line in run(["git", "ls-tree", "-r", "--full-tree", "HEAD"], self.workspace).stdout.splitlines():
            metadata, path = line.split("\t", 1)
            mode, kind, sha = metadata.split()
            entries.append(dict(mode=mode, type=kind, sha=sha, path=path))
        state_path = self.workspace / ".github" / "azurlane-state"
        if state_missing:
            state_path.unlink(missing_ok=True)
        else:
            put(self.workspace, ".github/azurlane-state",
                f"upstream_sha: {old}\ngenerator_hash: {old_hash or fingerprint(self.workspace)}\n")
        fixture = dict(tree=list(reversed(entries)), truncated=truncated)
        runner = r'''
const fs = require("fs");
const fixture = JSON.parse(fs.readFileSync(0, "utf8"));
const outputs = {};
const core = {setOutput: (key, value) => outputs[key] = value, info: () => {}, warning: () => {}};
const context = {repo: {owner: "fixture", repo: "fixture"}, sha: "fixture"};
const github = {rest: {
  git: {getTree: async () => ({data: {tree: fixture.tree, truncated: fixture.truncated}})},
}};
(async () => {
''' + script + r'''
  process.stdout.write(JSON.stringify(outputs));
})().catch(error => { console.error(error); process.exit(1); });
'''
        result = run(["node", "-e", runner], self.workspace, input=json.dumps(fixture),
                     env={"UPSTREAM_SHA": latest, "FORCE_FULL": str(force).lower()})
        return json.loads(result.stdout)

    def test_unchanged_fingerprint_skips_build(self) -> None:
        result = self.preflight()
        self.assertEqual(result["needs_build"], "false")
        self.assertEqual(result["force_full"], "false")

    def test_new_upstream_uses_incremental(self) -> None:
        result = self.preflight(latest="b" * 40)
        self.assertEqual(result["needs_build"], "true")
        self.assertEqual(result["force_full"], "false")

    def test_generator_change_forces_full(self) -> None:
        self.assertEqual(self.preflight(old_hash="f" * 64)["force_full"], "true")

    def test_explicit_full_is_preserved(self) -> None:
        self.assertEqual(self.preflight(force=True)["force_full"], "true")

    def test_missing_or_truncated_evidence_fails_safe(self) -> None:
        for kwargs in ({"truncated": True}, {"state_missing": True}, {"latest": ""}):
            with self.subTest(**kwargs):
                result = self.preflight(**kwargs)
                self.assertEqual(result["needs_build"], "true")
                self.assertEqual(result["force_full"], "true")


class PlannerTests(FixtureTest):
    def setUp(self) -> None:
        super().setUp()
        self.upstream = self.base / "upstream"
        init_repo(self.upstream)
        run(["git", "config", "uploadpack.allowFilter", "true"], self.upstream)
        for n in range(3):
            put(self.upstream, "history.txt", str(n))
            commit(self.upstream)
        for region in REGIONS:
            put(self.upstream, f"{region}/sharecfg/normal.lua", "return { id = 1 }\n")
            put(self.upstream, f"versions/{region}.txt", "1.2.3\n")
        put(self.upstream, "CN/const.lua", "SYSTEM_DUEL = 3\n")
        put(self.upstream, "CN/model/const/shiptype.lua", 'slot0 = class("ShipType")\nslot0.QuZhu = 1\n')
        put(self.upstream, "JP/gamecfg/skill/one.lua", "return {}\n")
        put(self.upstream, "JP/gamecfg/skill/two.lua", "return {}\n")
        self.previous = commit(self.upstream)
        self.runner = self.base / "runner"
        self.runner.mkdir()

    def plan(self, latest: str, *, previous: str | None = None, force: bool = False,
             remote: str | None = None) -> tuple[dict, dict[str, str]]:
        put(self.workspace, ".github/azurlane-state",
            f"upstream_sha: {previous or self.previous}\ngenerator_hash: {fingerprint(self.workspace)}\n")
        output = self.base / "outputs"
        output.write_text("")
        result = run(["bash", str(ROOT / "ci/sync-upstream.sh")], self.workspace, env={
            "UPSTREAM_SHA": latest, "GITHUB_OUTPUT": str(output), "RUNNER_TEMP": str(self.runner),
            "UPSTREAM_REMOTE": remote or self.upstream.as_uri(), "FORCE_FULL": str(force).lower(),
        }, check=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        outputs = dict(line.split("=", 1) for line in output.read_text().splitlines())
        plan_path = Path(outputs["plan_path"])
        return (json.loads(plan_path.read_text()) if plan_path.exists() else {}), outputs

    def test_alias_additions(self) -> None:
        for name in ALIASES:
            put(self.upstream, f"JP/sharecfg/{name}.lua", "return {}\n")
        plan, _ = self.plan(commit(self.upstream))
        self.assertEqual(plan["mode"], "incremental")
        self.assertEqual(plan["delete_outputs"], [])
        for target in ALIASES.values():
            self.assertIn(f"JP/ShareCfg/{target}.json", plan["output_paths"])

    def test_alias_modifications(self) -> None:
        for name in ALIASES:
            put(self.upstream, f"JP/sharecfg/{name}.lua", "return {}\n")
        self.previous = commit(self.upstream)
        for name in ALIASES:
            put(self.upstream, f"JP/sharecfg/{name}.lua", "return { id = 2 }\n")
        plan, _ = self.plan(commit(self.upstream))
        self.assertEqual(len(plan["required_source_paths"]), len(ALIASES) + 2)
        self.assertIn("CN/const.lua", plan["required_source_paths"])
        self.assertIn("CN/model/const/shiptype.lua", plan["required_source_paths"])
        self.assertEqual(plan["delete_outputs"], [])

    def test_alias_deletions(self) -> None:
        for name in ALIASES:
            put(self.upstream, f"JP/sharecfg/{name}.lua", "return {}\n")
        self.previous = commit(self.upstream)
        for name in ALIASES:
            (self.upstream / f"JP/sharecfg/{name}.lua").unlink()
        plan, _ = self.plan(commit(self.upstream))
        self.assertEqual(
            plan["required_source_paths"],
            ["CN/const.lua", "CN/model/const/shiptype.lua"],
        )
        self.assertEqual(len(plan["delete_outputs"]), 2 * len(ALIASES))

    def test_only_changed_region_is_checked_out(self) -> None:
        put(self.upstream, "JP/sharecfg/normal.lua", "return { id = 2 }\n")
        plan, outputs = self.plan(commit(self.upstream))
        self.assertEqual(
            plan["required_source_paths"],
            [
                "JP/sharecfg/normal.lua",
                "CN/const.lua",
                "CN/model/const/shiptype.lua",
            ],
        )
        source = Path(outputs["source_root"])
        self.assertTrue((source / "JP/sharecfg/normal.lua").is_file())
        self.assertFalse((source / "CN/sharecfg/normal.lua").exists())

    def test_shared_constants_are_fetched_for_lua_changes(self) -> None:
        put(self.upstream, "JP/sharecfg/normal.lua", "return { id = 2 }\n")
        plan, outputs = self.plan(commit(self.upstream))
        self.assertIn("CN/model/const/shiptype.lua", plan["required_source_paths"])
        self.assertIn("CN/const.lua", plan["required_source_paths"])
        self.assertTrue((Path(outputs["source_root"]) / "CN/model/const/shiptype.lua").is_file())
        self.assertTrue((Path(outputs["source_root"]) / "CN/const.lua").is_file())

    def test_shared_constants_change_falls_back_to_full(self) -> None:
        put(self.upstream, "CN/model/const/shiptype.lua", 'slot0 = class("ShipType")\nslot0.QuZhu = 2\n')
        plan, outputs = self.plan(commit(self.upstream))
        self.assertEqual(plan["mode"], "full")
        self.assertEqual(outputs["needs_sources"], "true")

    def test_gamecfg_rebuilds_whole_affected_category(self) -> None:
        put(self.upstream, "JP/gamecfg/skill/one.lua", "return { id = 2 }\n")
        plan, outputs = self.plan(commit(self.upstream))
        self.assertEqual(plan["gamecfg"], ["JP/GameCfg/skill.json"])
        source = Path(outputs["source_root"])
        self.assertTrue((source / "JP/gamecfg/skill/two.lua").is_file())
        self.assertFalse((source / "CN" / "sharecfg").exists())

    def test_gamecfg_records_changed_files_for_incremental_refresh(self) -> None:
        put(self.upstream, "JP/gamecfg/skill/one.lua", "return { id = 2 }\n")
        (self.upstream / "JP/gamecfg/skill/two.lua").unlink()
        plan, _ = self.plan(commit(self.upstream))
        self.assertEqual(plan["gamecfg_sources"], [
            {"path": "JP/gamecfg/skill/one.lua", "deleted": False},
            {"path": "JP/gamecfg/skill/two.lua", "deleted": True},
        ])

    def test_gamecfg_sources_empty_on_full_build(self) -> None:
        plan, _ = self.plan(self.previous, previous="d" * 40)
        self.assertEqual(plan["mode"], "full")
        self.assertEqual(plan["gamecfg_sources"], [])

    def test_versions_only_fetches_all_version_inputs(self) -> None:
        put(self.upstream, "versions/JP.txt", "1.2.4\n")
        plan, outputs = self.plan(commit(self.upstream))
        self.assertTrue(plan["versions"])
        self.assertEqual(plan["output_paths"], ["global/versions.json"])
        self.assertEqual(len(list((Path(outputs["source_root"]) / "versions").glob("*.txt"))), 5)

    def test_unchanged_never_connects_to_remote(self) -> None:
        _, outputs = self.plan(self.previous, remote=(self.base / "does-not-exist").as_uri())
        self.assertEqual(outputs["mode"], "unchanged")
        self.assertEqual(outputs["needs_processing"], "false")

    def test_unrelated_changes_are_noop(self) -> None:
        put(self.upstream, "README.md", "documentation only\n")
        plan, outputs = self.plan(commit(self.upstream))
        self.assertEqual(plan["mode"], "noop")
        self.assertEqual(outputs["needs_sources"], "false")
        self.assertEqual(outputs["needs_processing"], "true")

    def test_unknown_old_sha_falls_back_to_full(self) -> None:
        plan, outputs = self.plan(self.previous, previous="d" * 40)
        self.assertEqual(plan["mode"], "full")
        self.assertEqual(outputs["needs_sources"], "true")

    def test_fetch_omits_unneeded_commit_history(self) -> None:
        put(self.upstream, "JP/sharecfg/normal.lua", "return { id = 2 }\n")
        _, outputs = self.plan(commit(self.upstream))
        objects = run(["git", "cat-file", "--batch-all-objects", "--batch-check=%(objecttype)"],
                      Path(outputs["source_root"])).stdout.splitlines()
        self.assertEqual(objects.count("commit"), 2)


class VerifierTests(FixtureTest):
    def setUp(self) -> None:
        super().setUp()
        self.runner = self.base / "runner"
        self.out = self.runner / "amagi_data_generation"
        self.source = self.base / "lua"
        for region in REGIONS:
            (self.source / region / "sharecfg").mkdir(parents=True)
        generated = [f"JP/ShareCfg/table_{n}.json" for n in range(622)]
        helpers = ["global/versions.json"]
        self.report = dict(generated_files=generated, generated_helper_files=helpers)
        for rel in generated + helpers:
            put(self.out, rel)

    def verify(self, mode: str = "full", plan: dict | None = None) -> subprocess.CompletedProcess[str]:
        put(self.out, "generation-report.json", json.dumps(self.report))
        plan_path = self.base / "plan.json"
        plan_path.write_text(json.dumps(plan or {}))
        return run(["bash", str(ROOT / "ci/verify-generated.sh")], self.workspace, check=False, env={
            "RUNNER_TEMP": str(self.runner), "GITHUB_WORKSPACE": str(self.workspace),
            "AMAGI_UPSTREAM_ROOT": str(self.source), "AMAGI_MODE": mode,
            "AMAGI_INCREMENTAL_PLAN": str(plan_path),
        })

    def assert_rejected(self, result: subprocess.CompletedProcess[str]) -> None:
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_complete_full_output_passes(self) -> None:
        result = self.verify()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_full_rejects_declared_but_missing_output(self) -> None:
        (self.out / self.report["generated_files"][0]).unlink()
        self.assert_rejected(self.verify())

    def test_full_rejects_invalid_json(self) -> None:
        put(self.out, self.report["generated_files"][0], "{broken\n")
        self.assert_rejected(self.verify())

    def test_full_rejects_error_output(self) -> None:
        put(self.out, self.report["generated_files"][0], '{"__ERROR":"conversion failed"}\n')
        self.assert_rejected(self.verify())

    def test_full_rejects_missing_region_input(self) -> None:
        shutil.rmtree(self.source / "CN")
        self.assert_rejected(self.verify())

    def test_full_rejects_unreported_output(self) -> None:
        put(self.out, "JP/ShareCfg/unreported.json")
        self.assert_rejected(self.verify())

    def test_incremental_rejects_output_outside_plan(self) -> None:
        shutil.rmtree(self.out)
        rel = "JP/ShareCfg/unplanned.json"
        put(self.out, rel)
        self.report = dict(generated_files=[rel], generated_helper_files=[])
        self.assertNotEqual(self.verify("incremental", {"output_paths": []}).returncode, 0)

    def test_incremental_optional_pair_need_not_exist(self) -> None:
        shutil.rmtree(self.out)
        rel = "JP/ShareCfg/normal.json"
        put(self.out, rel)
        self.report = dict(generated_files=[rel], generated_helper_files=[])
        result = self.verify("incremental", {"output_paths": [rel, "JP/sharecfgdata/normal.json"]})
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


class CommitScriptTests(FixtureTest):
    def setUp(self) -> None:
        super().setUp()
        run(["git", "branch", "-M", "main"], self.workspace)
        self.origin = self.base / "origin.git"
        run(["git", "init", "-q", "--bare", str(self.origin)], self.base)
        run(["git", "remote", "add", "origin", str(self.origin)], self.workspace)
        put(self.workspace, "global/versions.json", json.dumps({"JP": "9.2.819", "CN": "9.7.380"}))
        commit(self.workspace)
        run(["git", "push", "-q", "origin", "main"], self.workspace)

    def commit_generated(self) -> subprocess.CompletedProcess[str]:
        return run(["bash", str(ROOT / "ci/commit-generated.sh")], self.workspace, check=False,
                   env={"GITHUB_REF_NAME": "main"})

    def head(self) -> str:
        return run(["git", "rev-parse", "HEAD"], self.workspace).stdout.strip()

    def subjects(self, count: int) -> list[str]:
        return run(["git", "log", f"-{count}", "--pretty=%s"], self.workspace).stdout.strip().splitlines()

    def files_in(self, rev: str) -> list[str]:
        return sorted(run(["git", "show", "--name-only", "--format=", rev], self.workspace).stdout.split())

    def test_region_change_commits_with_its_version_delta(self) -> None:
        put(self.workspace, "JP/ShareCfg/table.json", '{"v":2}\n')
        put(self.workspace, "global/versions.json", json.dumps({"JP": "9.2.821", "CN": "9.7.380"}))
        result = self.commit_generated()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.subjects(1), ["update [JP]: 9.2.819 -> 9.2.821 [skip ci]"])
        self.assertEqual(self.files_in("HEAD"), ["JP/ShareCfg/table.json", "global/versions.json"])

    def test_each_changed_region_gets_its_own_commit(self) -> None:
        put(self.workspace, "CN/ShareCfg/a.json", '{"v":2}\n')
        put(self.workspace, "JP/ShareCfg/b.json", '{"v":2}\n')
        put(self.workspace, "global/versions.json", json.dumps({"JP": "9.2.821", "CN": "9.7.381"}))
        result = self.commit_generated()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.subjects(2), [
            "update [JP]: 9.2.819 -> 9.2.821 [skip ci]",
            "update [CN]: 9.7.380 -> 9.7.381 [skip ci]",
        ])
        # Shared files ride with the last region commit only.
        self.assertEqual(self.files_in("HEAD~1"), ["CN/ShareCfg/a.json"])
        self.assertEqual(self.files_in("HEAD"), ["JP/ShareCfg/b.json", "global/versions.json"])

    def test_data_change_without_version_bump_still_commits(self) -> None:
        put(self.workspace, "CN/ShareCfg/a.json", '{"v":2}\n')
        result = self.commit_generated()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.subjects(1), ["update [CN]: 9.7.380 -> 9.7.380 [skip ci]"])

    def test_shared_only_changes_are_left_for_the_next_region_commit(self) -> None:
        before = self.head()
        put(self.workspace, "global/versions.json", json.dumps({"JP": "9.2.821", "CN": "9.7.380"}))
        result = self.commit_generated()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.head(), before)

    def test_no_working_tree_changes_skips_commit(self) -> None:
        before = self.head()
        result = self.commit_generated()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.head(), before)

if __name__ == "__main__":
    unittest.main()
