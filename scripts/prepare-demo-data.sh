:
# Prepare demo data for focus README recording.
# Run from the repository root.

set -e

REPO_ROOT="$(git rev-parse --show-toplevel)"
DB="${REPO_ROOT}/.focus/focus.db"
WT_NAME="feat-auth-design"
WT_PATH="${REPO_ROOT}/.worktrees/${WT_NAME}"

if [ ! -f "${DB}" ]; then
  echo "focus database not found at ${DB}. Run focus once first."
  exit 1
fi

echo "Preparing demo data in ${REPO_ROOT} ..."

# Create a real git worktree for the demo phase if it doesn't exist.
if [ ! -d "${WT_PATH}" ]; then
  git worktree add -b "${WT_NAME}" "${WT_PATH}" || git worktree add "${WT_PATH}" "${WT_NAME}" || true
fi

# Reset the demo worktree so every recording starts from a clean state.
git -C "${WT_PATH}" checkout -- . >/dev/null 2>&1 || true
git -C "${WT_PATH}" clean -fd >/dev/null 2>&1 || true
rm -f "${WT_PATH}/docs/adr/012-jwt-authentication.md"

# Clean up any previous demo tasks.
sqlite3 "${DB}" <<SQL
DELETE FROM task_dependencies WHERE from_task_id LIKE 'demo-%' OR to_task_id LIKE 'demo-%';
DELETE FROM worktree_contexts WHERE worktree_id = '${WT_PATH}';
DELETE FROM task_contexts WHERE id LIKE 'demo-%';

INSERT INTO task_contexts (id, repo_id, title, goal, state, priority, parent_task_id, preferred_worktree_id, created_at, updated_at)
VALUES
  ('demo-phase-auth', '${REPO_ROOT}', 'Design authentication system', 'Design a secure JWT-based authentication system for the API', 'active', 'high', NULL, '${WT_PATH}', datetime('now'), datetime('now')),
  ('demo-step-research', '${REPO_ROOT}', 'Research JWT patterns', 'Survey current JWT best practices and libraries', 'active', 'medium', 'demo-phase-auth', '${WT_PATH}', datetime('now'), datetime('now')),
  ('demo-step-schema', '${REPO_ROOT}', 'Design token schema', 'Define claims, expiry, refresh strategy and key rotation', 'blocked', 'medium', 'demo-phase-auth', '${WT_PATH}', datetime('now', '+1 minute'), datetime('now', '+1 minute')),
  ('demo-step-api', '${REPO_ROOT}', 'Implement login API', 'Build /login and /refresh endpoints with validation', 'blocked', 'medium', 'demo-phase-auth', '${WT_PATH}', datetime('now', '+2 minutes'), datetime('now', '+2 minutes'));

INSERT INTO task_dependencies (from_task_id, to_task_id, dependency_type)
VALUES
  ('demo-step-research', 'demo-step-schema', 'hard'),
  ('demo-step-schema', 'demo-step-api', 'hard');

INSERT INTO worktree_contexts (worktree_id, repo_id, primary_task_id, task_mode, task_name, branch_snapshot, last_active_at)
VALUES
  ('${WT_PATH}', '${REPO_ROOT}', 'demo-phase-auth', 'single', 'Design authentication system', '${WT_NAME}', datetime('now'));
SQL

echo "Demo data ready."
echo "DAG: demo-phase-auth -> demo-step-research -> demo-step-schema -> demo-step-api"
echo "Worktree: ${WT_PATH}"
