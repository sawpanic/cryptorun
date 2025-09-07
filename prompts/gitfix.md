“DEPLOY NOT UPDATING” WHEN DEFAULT BRANCH = reset/main-clean
HOUSE RULES

OUTPUT DISCIPLINE — DO NOT EXCEED CONSOLE LIMITS. Never compress; if output would overflow, auto-continue with clear headers and resume precisely.

DOCS MANDATE — UPDATE MD ON EVERY PROMPT. Any config/code change must ship with docs and a CHANGELOG entry in this run.

ALREADY IMPLEMENTED PRE-FLIGHT. Before modifying anything, search repo for existing branch-name fixes. If already done or substantially complete:

STOP. Do not re-implement.

Emit an ALREADY IMPLEMENTED alert with exact file paths/lines and a concise Diff Plan (smallest additional edits only).

WRITE-SCOPE. Patch-only unless a new file is explicitly required. Minimal diffs. Preserve style/format.

CITATIONS & EVIDENCE. For every conclusion, show exact file paths and relevant line ranges from the repo; provide GitHub UI URLs where applicable.

STYLE. Plain English, no unexplained jargon.

OBJECTIVE

Diagnose and fix why deploys aren’t updating when the default branch is reset/main-clean (non-standard). Ensure:

remote default and upstream are correctly set to reset/main-clean,

all GitHub Actions/Pages workflows trigger on reset/main-clean,

branch protection and environment rules point to reset/main-clean,

a deterministic verification plan passes.

PHASE 0 — PRE-FLIGHT “ALREADY IMPLEMENTED” SCAN

Search:

Files: .github/workflows/*.yml, docs/DEPLOY*.md, README*, PAGES*.md, ACTIONS*.md, infra/*, deploy/*, scripts/*.

Look for occurrences of: main, master, default branch, reset/main-clean, branches:, on: push, workflow_dispatch, concurrency, permissions, pages, deployment, environment:, environments:, required_reviewers, gh-pages.

Goal: detect prior edits aligning triggers to reset/main-clean.

If found complete: Output ALREADY IMPLEMENTED with file paths + lines; provide a short Diff Plan only (e.g., a missing workflow_dispatch). Stop further phases.

ACCEPTANCE

Either emit ALREADY IMPLEMENTED with evidence and stop, or proceed (not implemented).

PHASE 1 — REMOTE & DEFAULT BRANCH WIRING

Collect evidence (don’t run; show commands & expected outputs):

git remote -v

git remote get-url origin

Resolve default remote branch: git symbolic-ref refs/remotes/origin/HEAD (should point to origin/reset/main-clean)

Current branch: git rev-parse --abbrev-ref HEAD

Upstream: git rev-parse --abbrev-ref --symbolic-full-name @{u} (expect origin/reset/main-clean)

Sync status:

git status --porcelain

git log --oneline origin/reset/main-clean..HEAD (ahead)

git log --oneline HEAD..origin/reset/main-clean (behind)

Diagnose & prescribe fixes (show exact commands):

If origin/HEAD not set →
git remote set-head origin -a
(then verify it resolves to origin/reset/main-clean; if not, force)
git remote set-head origin reset/main-clean

If branch isn’t tracking upstream →
git branch -u origin/reset/main-clean
or on first push: git push -u origin reset/main-clean

If ahead → git push

If behind & safe → git pull --ff-only

ACCEPTANCE

Clear statement that origin default and local upstream both target reset/main-clean, with evidence.

GIT COMMIT CHECKLIST (PHASE 1)

If you add docs/TROUBLESHOOT-DEPLOY.md with this branch wiring guidance:
Commit message: docs: add deploy triage and default-branch wiring notes

PHASE 2 — WORKFLOW TRIGGER ALIGNMENT (ALL YAML)

Enumerate all workflows: .github/workflows/*.yml. For each:

Extract name, and all on: triggers (push, pull_request, workflow_dispatch, release, schedule, tags, branches, paths/paths-ignore).

Rule: any workflow that must run on default branch changes must explicitly include branches: [ reset/main-clean ] (or omit branches entirely if the intent is “any branch”). If it currently says main or master, that’s the likely break.

Ensure every deployment-relevant workflow supports manual run:
on: { workflow_dispatch: {} }

Produce minimal unified diffs for each affected file, e.g.:

--- a/.github/workflows/deploy.yml
+++ b/.github/workflows/deploy.yml
@@
 on:
   push:
-    branches: [ main ]
+    branches: [ reset/main-clean ]
+  workflow_dispatch: {}


If multiple workflows reference main/master, patch them all. If a workflow is intended for tags only, leave branches untouched.

ACCEPTANCE

A table listing each workflow file → current branch filters → proposed branch filters.

Minimal diffs prepared for each file.

GIT COMMIT CHECKLIST (PHASE 2)

ci(workflows): align branch triggers to reset/main-clean; add workflow_dispatch

PHASE 3 — GITHUB PAGES (IF USED)

Detect Pages usage:

Workflow uses actions/deploy-pages or file names like pages.yml, deploy-pages.yml.

gh-pages branch or Pages settings mention “Build and deploy from GitHub Actions”.

Fixes (minimal diffs):

Ensure proper permissions:

 permissions:
-  contents: read
+  contents: read
+  pages: write
+  id-token: write


Ensure concurrency for pages:

 concurrency:
-  group: ci
+  group: "pages"
   cancel-in-progress: true


Ensure correct artifact name if upload-pages-artifact/deploy-pages expect it.

Ensure on.push.branches (if present) includes reset/main-clean.

ACCEPTANCE

Pages workflow(s) patched with minimal diffs and short rationale.

GIT COMMIT CHECKLIST (PHASE 3)

ci(pages): fix permissions/concurrency; align branch to reset/main-clean

PHASE 4 — ACTIONS → ENVIRONMENTS (NON-PAGES)

If deployment uses environments (e.g., staging, production):

Checks & minimal fixes:

jobs.<name>.environment.name must match an existing environment.

If approvals required, document the UI path and reviewers; do not remove protections without explicit intent.

Ensure permissions for deployments/OIDC:

 permissions:
-  contents: read
+  contents: read
+  deployments: write
+  id-token: write


Add manual trigger for retries:

 on:
   push:
     branches: [ reset/main-clean ]
+  workflow_dispatch: {}


Add safe concurrency to prevent stuck runs:

 concurrency:
-  group: deploy
+  group: deploy-${{ github.ref }}
   cancel-in-progress: true


ACCEPTANCE

Unified diffs showing exact changes; quick note on secrets names referenced (no secret values).

GIT COMMIT CHECKLIST (PHASE 4)

ci(deploy): add workflow_dispatch + concurrency + permissions for env deploys

PHASE 5 — PATH FILTERS & PROTECTION RULES

Path filters

If workflows have paths: and recent changes didn’t touch those paths, they won’t trigger. Provide two options with diffs:

Widen paths: to include real change locations, or

Remove paths: for deploy-critical jobs.

Example minimal widening:

 on:
   push:
     branches: [ reset/main-clean ]
-    paths: [ "frontend/**" ]
+    paths: [ "frontend/**", "infra/**", ".github/workflows/**", "docs/**" ]


Branch protection

If protection requires checks named for main, update rules to reference checks on reset/main-clean (document the UI path and exact checkbox names). Do not change rules in code; provide instructions and URLs.

ACCEPTANCE

Show lines where paths/branches appear and the proposed minimal change.

GIT COMMIT CHECKLIST (PHASE 5)

ci: adjust path filters to match actual change surfaces

PHASE 6 — REMEDIATION BUNDLE

Output three artifacts:

(A) Patch set (unified diffs)

All workflow YAML edits from Phases 2–5.

(B) docs/DEPLOY.md (create or update)
Include:

Default branch is reset/main-clean (callout box).

Trigger matrix (push/tags/manual) and which workflows deploy what.

Pages vs Environment deploy flow.

How to force a rebuild (empty commit or manual dispatch).

Common failure modes (path filters; missing permissions; environment approvals; concurrency; wrong branch).

(C) CHANGELOG entry
Under Unreleased:

fix(ci): align deploy triggers to reset/main-clean; add manual dispatch; adjust permissions/concurrency; docs

ACCEPTANCE

(A) full diffs, (B) rendered DEPLOY.md content, (C) CHANGELOG lines.

GIT COMMIT CHECKLIST (PHASE 6)

ci/docs: finalize deploy remediation; update CHANGELOG

PHASE 7 — VERIFICATION & ROLLBACK

Verification (choose appropriate track):

If Pages:

Manual dispatch the Pages workflow (provide the exact Actions URL).

Expected job order: build → upload-pages-artifact → deploy-pages.

Provide the “View deployment” link; advise cache-busting with ?v=<commit-sha>.

If Environments:

Manual dispatch for the deploy workflow.

If approval required, name the environment and UI path to approve.

Provide the Deployments page URL; list expected status checks.

Rollback plan:

git revert the workflow commit if needed.

Restore previous Pages setting / environment rules (list exact settings pages).

How to re-enable original path filters if too broad.

Pass/Fail checklist:

Workflow triggers on a commit to reset/main-clean.

Run completes; deployment record shows latest commit SHA.

Artifact is published (Pages) or environment shows updated version.

DELIVERABLES SUMMARY (WHAT TO OUTPUT)

Phase-by-phase evidence with file paths/line refs.

Minimal unified diffs for each workflow fix.

New/updated docs/DEPLOY.md and CHANGELOG entry text.

A single Remediation Commands block (branch → commit → push → open PR).

Verification steps with clickable URLs and rollback notes.

REMEDIATION COMMANDS (REFERENCE ONLY — DO NOT EXECUTE)

Provide, ready to copy:

git checkout -b fix/deploy-branch-alignment
# apply changes
git add -A
git commit -m "ci: align deploy triggers to reset/main-clean; add manual dispatch; pages/env perms & concurrency; docs"
git push -u origin fix/deploy-branch-alignment
# If GitHub CLI is available:
gh pr create --fill --base reset/main-clean --title "Fix: Deploy updates on reset/main-clean" --body "See DEPLOY.md; workflow diffs included."
# Else provide the PR creation URL for the repo.
