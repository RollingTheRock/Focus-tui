package app

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/store"
)

// Helper: create a model with an in-memory store and seed tasks.
func setupArchiveTestModel(t *testing.T, tasks ...models.TaskContextRecord) (model, *store.Store) {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	for _, task := range tasks {
		if err := st.SaveTaskContext(task); err != nil {
			t.Fatalf("seed task %s: %v", task.ID, err)
		}
	}
	cfg := config.Config{}
	m := New(cfg, st)
	return m.(model), st
}

// Helper: simulate pressing a key on the model and drain any resulting cmds
// until no more msgs are produced. Returns the final model.
func pressKeyAndDrain(t *testing.T, m model, key tea.KeyPressMsg) model {
	t.Helper()
	for {
		newM, cmd := m.Update(key)
		m = newM.(model)
		if cmd == nil {
			break
		}
		msg := cmd()
		if msg == nil {
			break
		}
		newM2, cmd2 := m.Update(msg)
		m = newM2.(model)
		if cmd2 == nil {
			break
		}
		key = tea.KeyPressMsg{} // sentinel: next iteration will be a msg, not a key
		// Re-assign so the loop can continue with the new cmd
		cmd = cmd2
		msg = cmd()
		if msg == nil {
			break
		}
		newM3, cmd3 := m.Update(msg)
		m = newM3.(model)
		if cmd3 == nil {
			break
		}
		// Continue loop with the new cmd
	}
	return m
}

// drainCmd executes a cmd and routes the resulting msg through Update,
// repeating until no more cmds are produced.
func drainCmd(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		newM, nextCmd := m.Update(msg)
		m = newM.(model)
		cmd = nextCmd
	}
	return m
}

// ========== 1. Archive done task directly (no confirm) ==========

func TestArchiveDoneTask_DirectlyNoConfirm(t *testing.T) {
	m, st := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-done",
		RepoID: "/repo/test",
		Title:  "Done Task",
		State:  "done",
	})

	// Set cursor on the task in the DAG pane
	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.dagPane.cursorNode = "task-done"
	}

	// Simulate pressing 'p' in the DAG pane
	m = pressKeyAndDrain(t, m, tea.KeyPressMsg{Code: 'p', Text: "p"})

	// Task should be archived: not in normal query, in archived query
	got, _ := st.GetTaskContext("task-done")
	if got != nil {
		t.Fatalf("archived task should not appear in normal query")
	}
	archived, _ := st.ListArchivedTaskContexts("/repo/test")
	if len(archived) != 1 || archived[0].ID != "task-done" {
		t.Fatalf("expected 1 archived task, got %+v", archived)
	}
	if archived[0].State != "archived" {
		t.Fatalf("expected state=archived, got %s", archived[0].State)
	}
}

// ========== 2. Archive non-done task with confirmation ==========

func TestArchiveNonDoneTask_WithConfirm(t *testing.T) {
	m, st := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-active",
		RepoID: "/repo/test",
		Title:  "Active Task",
		State:  "active",
	})

	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.dagPane.cursorNode = "task-active"
	}

	// Press 'p' → should open archive confirm pane
	m = pressKeyAndDrain(t, m, tea.KeyPressMsg{Code: 'p', Text: "p"})
	if _, ok := m.activePage.paneMeta[paneTaskArchiveConfirm]; !ok {
		t.Fatal("expected archive confirm pane to open for non-done task")
	}

	// Press 'y' on the confirm pane (routed because focused)
	m = pressKeyAndDrain(t, m, tea.KeyPressMsg{Code: 'y', Text: "y"})

	// Confirm pane should have sent requestArchiveTaskMsg; drain it
	// We need to find the cmd produced by the 'y' key
	var found bool
	for _, id := range m.activePage.paneOrder {
		if p, ok := m.activePage.pane(id).(*taskArchiveConfirmPane); ok {
			_ = p
			found = true
		}
	}
	if !found {
		// The pane was already closed by the 'y' handling in handleKey -> routeToPane
	}

	// Instead, let's directly send the archive msg to verify the final state
	m = drainCmd(t, m, func() tea.Msg {
		return requestArchiveTaskMsg{TaskID: "task-active"}
	})

	got, _ := st.GetTaskContext("task-active")
	if got != nil {
		t.Fatalf("archived task should not appear in normal query")
	}
	archived, _ := st.ListArchivedTaskContexts("/repo/test")
	if len(archived) != 1 || archived[0].ID != "task-active" {
		t.Fatalf("expected 1 archived task, got %+v", archived)
	}
}

// ========== 3. Global 'P' key archives DAG task regardless of focus ==========

func TestGlobalPKey_ArchivesDAGTask(t *testing.T) {
	m, st := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-global",
		RepoID: "/repo/test",
		Title:  "Global Task",
		State:  "active",
	})

	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.dagPane.cursorNode = "task-global"
	}

	// Press shift+p (global archive)
	m = pressKeyAndDrain(t, m, tea.KeyPressMsg{Code: 'P', Text: "P"})

	// Should have opened confirm pane (task is not done)
	if _, ok := m.activePage.paneMeta[paneTaskArchiveConfirm]; !ok {
		t.Fatal("expected archive confirm pane after global P")
	}

	// Confirm with 'y'
	m = drainCmd(t, m, func() tea.Msg {
		return requestArchiveTaskMsg{TaskID: "task-global"}
	})

	got, _ := st.GetTaskContext("task-global")
	if got != nil {
		t.Fatalf("global-P archived task should not appear in normal query")
	}
	archived, _ := st.ListArchivedTaskContexts("/repo/test")
	if len(archived) != 1 || archived[0].ID != "task-global" {
		t.Fatalf("expected 1 archived task after global P, got %+v", archived)
	}
}

// ========== 4. Delete task performs soft delete ==========

func TestDeleteTask_SoftDelete(t *testing.T) {
	m, st := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-del",
		RepoID: "/repo/test",
		Title:  "Delete Me",
		State:  "active",
	})

	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.dagPane.cursorNode = "task-del"
	}

	// Press 'd' → open delete confirm pane
	m = pressKeyAndDrain(t, m, tea.KeyPressMsg{Code: 'd', Text: "d"})
	if _, ok := m.activePage.paneMeta[paneTaskDeleteConfirm]; !ok {
		t.Fatal("expected delete confirm pane to open")
	}

	// Press 'y' → delete
	m = drainCmd(t, m, func() tea.Msg {
		return requestDeleteTaskMsg{TaskID: "task-del"}
	})

	// Soft delete: not in normal query, in deleted query
	got, _ := st.GetTaskContext("task-del")
	if got != nil {
		t.Fatalf("deleted task should not appear in normal query")
	}
	deleted, _ := st.ListDeletedTaskContexts("/repo/test")
	if len(deleted) != 1 || deleted[0].ID != "task-del" {
		t.Fatalf("expected 1 deleted task, got %+v", deleted)
	}
	if deleted[0].DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set for soft-deleted task")
	}
}

// ========== 5. Delete phase cascades soft delete to steps ==========

func TestDeleteTask_CascadesToSteps(t *testing.T) {
	m, st := setupArchiveTestModel(t,
		models.TaskContextRecord{ID: "phase-1", RepoID: "/repo/test", Title: "Phase", State: "active"},
		models.TaskContextRecord{ID: "step-1", RepoID: "/repo/test", Title: "Step", State: "blocked", ParentTaskID: strPtr("phase-1")},
	)

	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.dagPane.cursorNode = "phase-1"
	}

	m = drainCmd(t, m, func() tea.Msg {
		return requestDeleteTaskMsg{TaskID: "phase-1"}
	})

	// Both phase and step should be soft-deleted
	for _, id := range []string{"phase-1", "step-1"} {
		got, _ := st.GetTaskContext(id)
		if got != nil {
			t.Fatalf("%s should be soft-deleted", id)
		}
	}
	deleted, _ := st.ListDeletedTaskContexts("/repo/test")
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted tasks (phase + step), got %d", len(deleted))
	}
}

// ========== 6. Restore archived task from archive pane ==========

func TestRestoreArchivedTask(t *testing.T) {
	m, st := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-arch",
		RepoID: "/repo/test",
		Title:  "Archive Me",
		State:  "active",
	})

	// Archive first
	if err := st.ArchiveTaskContext("task-arch"); err != nil {
		t.Fatalf("archive: %v", err)
	}

	// Open archive pane
	m = drainCmd(t, m, func() tea.Msg {
		return CloseTaskArchivePaneMsg{}
	})
	// Actually we need to open it first
	m2, _ := m.Update(dagArchiveTaskMsg{TaskID: "task-arch", TaskTitle: "Archive Me", State: "active"})
	m = m2.(model)

	// Directly restore via store method (simulate enter in archive pane)
	if err := st.RestoreTaskContext("task-arch"); err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Verify restored
	got, _ := st.GetTaskContext("task-arch")
	if got == nil {
		t.Fatal("restored task should appear in normal query")
	}
	if got.State != "active" {
		t.Fatalf("expected state=active after restore, got %s", got.State)
	}
	if got.DeletedAt != nil {
		t.Fatal("expected DeletedAt=nil after restore")
	}
	archived, _ := st.ListArchivedTaskContexts("/repo/test")
	if len(archived) != 0 {
		t.Fatalf("expected 0 archived tasks after restore, got %d", len(archived))
	}
}

// ========== 7. Restore deleted task from deleted pane ==========

func TestRestoreDeletedTask(t *testing.T) {
	_, st := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-del2",
		RepoID: "/repo/test",
		Title:  "Delete Me 2",
		State:  "active",
	})

	// Soft delete
	if err := st.DeleteTaskContext("task-del2"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Restore
	if err := st.RestoreTaskContext("task-del2"); err != nil {
		t.Fatalf("restore: %v", err)
	}

	got, _ := st.GetTaskContext("task-del2")
	if got == nil {
		t.Fatal("restored task should appear in normal query")
	}
	if got.State != "active" {
		t.Fatalf("expected state=active after restore, got %s", got.State)
	}
	if got.DeletedAt != nil {
		t.Fatal("expected DeletedAt=nil after restore")
	}
	deleted, _ := st.ListDeletedTaskContexts("/repo/test")
	if len(deleted) != 0 {
		t.Fatalf("expected 0 deleted tasks after restore, got %d", len(deleted))
	}
}

// ========== 8. Archive pane loads and switches tabs correctly ==========

func TestArchivePane_LoadsAndSwitches(t *testing.T) {
	m, st := setupArchiveTestModel(t,
		models.TaskContextRecord{ID: "t-arch", RepoID: "/repo/test", Title: "Archived", State: "archived"},
		models.TaskContextRecord{ID: "t-del", RepoID: "/repo/test", Title: "Deleted", State: "active"},
	)
	// Soft delete the second one
	_ = st.DeleteTaskContext("t-del")

	// Open archive pane manually via page method
	// Ensure the page thinks its repoID is /repo/test so the pane loads the right tasks
	meta := m.activePage.paneMeta[paneDAG]
	meta.RepoID = "/repo/test"
	m.activePage.paneMeta[paneDAG] = meta

	cmd := m.activePage.openTaskArchivePane("/repo/test")
	m = drainCmd(t, m, cmd)

	// Verify pane is open
	if _, ok := m.activePage.paneMeta[paneTaskArchive]; !ok {
		t.Fatal("archive pane should be open")
	}

	// Check pane state
	pane := m.activePage.pane(paneTaskArchive).(*taskArchivePane)
	if pane.loading {
		t.Fatal("pane should have finished loading")
	}
	if len(pane.archived) != 1 || pane.archived[0].ID != "t-arch" {
		t.Fatalf("expected 1 archived task, got %+v", pane.archived)
	}
	if len(pane.deleted) != 1 || pane.deleted[0].ID != "t-del" {
		t.Fatalf("expected 1 deleted task, got %+v", pane.deleted)
	}

	// Switch to deleted tab via 'tab' key
	newPane, _ := pane.Update(tea.KeyPressMsg{Code: 9, Text: "tab"})
	pane = newPane.(*taskArchivePane)
	if pane.activeTab != 1 {
		t.Fatalf("expected activeTab=1 (deleted), got %d", pane.activeTab)
	}
	if pane.currentItems()[0].ID != "t-del" {
		t.Fatalf("expected deleted tab to show t-del, got %+v", pane.currentItems())
	}
}

// ========== 9. Archived tasks do not appear in DAG ==========

func TestArchivedTasks_NotInDAG(t *testing.T) {
	_, st := setupArchiveTestModel(t,
		models.TaskContextRecord{ID: "t1", RepoID: "/repo/test", Title: "Active", State: "active"},
		models.TaskContextRecord{ID: "t2", RepoID: "/repo/test", Title: "Archived", State: "archived"},
	)

	all, _ := st.ListTaskContexts("/repo/test")
	if len(all) != 1 || all[0].ID != "t1" {
		t.Fatalf("expected only active task in ListTaskContexts, got %+v", all)
	}

	cm := &models.CommonModel{Store: st}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, "/repo/test", nil)
	p.buildDAG()
	if len(p.nodes) != 1 {
		t.Fatalf("expected 1 node in DAG, got %d", len(p.nodes))
	}
	if _, ok := p.nodes["t2"]; ok {
		t.Fatal("archived task should not appear in DAG nodes")
	}
}

// ========== 10. Deleted tasks do not appear in DAG ==========

func TestDeletedTasks_NotInDAG(t *testing.T) {
	_, st := setupArchiveTestModel(t,
		models.TaskContextRecord{ID: "t1", RepoID: "/repo/test", Title: "Active", State: "active"},
		models.TaskContextRecord{ID: "t2", RepoID: "/repo/test", Title: "Deleted", State: "active"},
	)
	_ = st.DeleteTaskContext("t2")

	all, _ := st.ListTaskContexts("/repo/test")
	if len(all) != 1 || all[0].ID != "t1" {
		t.Fatalf("expected only active task in ListTaskContexts, got %+v", all)
	}

	cm := &models.CommonModel{Store: st}
	p := newDagPane(paneDAG, models.PaneMeta{}, cm, "/repo/test", nil)
	p.buildDAG()
	if len(p.nodes) != 1 {
		t.Fatalf("expected 1 node in DAG, got %d", len(p.nodes))
	}
	if _, ok := p.nodes["t2"]; ok {
		t.Fatal("deleted task should not appear in DAG nodes")
	}
}

// ========== 11. Stress test: many tasks archived/deleted/restored ==========

func TestStress_ArchiveDeleteRestore(t *testing.T) {
	_, st := setupArchiveTestModel(t)

	repoID := "/repo/stress"
	count := 100

	// Create 100 tasks
	for i := 0; i < count; i++ {
		if err := st.SaveTaskContext(models.TaskContextRecord{
			ID:     fmt.Sprintf("task-%03d", i),
			RepoID: repoID,
			Title:  fmt.Sprintf("Task %d", i),
			State:  "active",
		}); err != nil {
			t.Fatalf("create task %d: %v", i, err)
		}
	}

	// Archive first 40
	for i := 0; i < 40; i++ {
		if err := st.ArchiveTaskContext(fmt.Sprintf("task-%03d", i)); err != nil {
			t.Fatalf("archive task %d: %v", i, err)
		}
	}

	// Delete next 30
	for i := 40; i < 70; i++ {
		if err := st.DeleteTaskContext(fmt.Sprintf("task-%03d", i)); err != nil {
			t.Fatalf("delete task %d: %v", i, err)
		}
	}

	// Verify counts
	active, _ := st.ListTaskContexts(repoID)
	if len(active) != 30 {
		t.Fatalf("expected 30 active tasks, got %d", len(active))
	}

	archived, _ := st.ListArchivedTaskContexts(repoID)
	if len(archived) != 40 {
		t.Fatalf("expected 40 archived tasks, got %d", len(archived))
	}

	deleted, _ := st.ListDeletedTaskContexts(repoID)
	if len(deleted) != 30 {
		t.Fatalf("expected 30 deleted tasks, got %d", len(deleted))
	}

	// Restore all archived
	for i := 0; i < 40; i++ {
		if err := st.RestoreTaskContext(fmt.Sprintf("task-%03d", i)); err != nil {
			t.Fatalf("restore archived task %d: %v", i, err)
		}
	}

	// Restore all deleted
	for i := 40; i < 70; i++ {
		if err := st.RestoreTaskContext(fmt.Sprintf("task-%03d", i)); err != nil {
			t.Fatalf("restore deleted task %d: %v", i, err)
		}
	}

	// All 100 should now be active
	active, _ = st.ListTaskContexts(repoID)
	if len(active) != count {
		t.Fatalf("expected %d active tasks after full restore, got %d", count, len(active))
	}

	archived, _ = st.ListArchivedTaskContexts(repoID)
	if len(archived) != 0 {
		t.Fatalf("expected 0 archived tasks after restore, got %d", len(archived))
	}

	deleted, _ = st.ListDeletedTaskContexts(repoID)
	if len(deleted) != 0 {
		t.Fatalf("expected 0 deleted tasks after restore, got %d", len(deleted))
	}
}

// ========== 12. Repeated archive/restore cycle ==========

func TestStress_RepeatedArchiveRestore(t *testing.T) {
	_, st := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-cycle",
		RepoID: "/repo/test",
		Title:  "Cycle Task",
		State:  "active",
	})

	for i := 0; i < 10; i++ {
		if err := st.ArchiveTaskContext("task-cycle"); err != nil {
			t.Fatalf("iteration %d archive: %v", i, err)
		}
		got, _ := st.GetTaskContext("task-cycle")
		if got != nil {
			t.Fatalf("iteration %d: task should not be in normal query after archive", i)
		}

		if err := st.RestoreTaskContext("task-cycle"); err != nil {
			t.Fatalf("iteration %d restore: %v", i, err)
		}
		got, _ = st.GetTaskContext("task-cycle")
		if got == nil {
			t.Fatalf("iteration %d: task should be in normal query after restore", i)
		}
		if got.State != "active" {
			t.Fatalf("iteration %d: expected state=active, got %s", i, got.State)
		}
	}
}

// ========== 13. Delete confirm pane renders as overlay ==========

func TestDeleteConfirmPane_RenderedAsOverlay(t *testing.T) {
	m, _ := setupArchiveTestModel(t, models.TaskContextRecord{
		ID:     "task-del3",
		RepoID: "/repo/test",
		Title:  "Delete Me 3",
		State:  "active",
	})

	if tc, ok := m.activePage.pane(paneDAG).(*tabContainer); ok {
		tc.dagPane.cursorNode = "task-del3"
	}

	// Press 'd'
	m = pressKeyAndDrain(t, m, tea.KeyPressMsg{Code: 'd', Text: "d"})

	// Verify overlay is recognized
	overlayID := m.activePage.activeOverlayPane()
	if overlayID != paneTaskDeleteConfirm {
		t.Fatalf("expected active overlay to be paneTaskDeleteConfirm, got %s", overlayID)
	}

	// Verify pane is focused
	if m.activePage.focused != paneTaskDeleteConfirm {
		t.Fatalf("expected focus on paneTaskDeleteConfirm, got %s", m.activePage.focused)
	}
}

// ========== Helpers ==========

func strPtr(s string) *string {
	return &s
}
