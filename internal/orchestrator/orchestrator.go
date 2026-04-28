package orchestrator

import "fmt"

type Task struct {
	ID         string
	Name       string
	PlanID     string
	WorktreeID string
	Provider   string
}

type Store interface {
	GetDownstreamTasks(taskID string) ([]Task, error)
	AllPrerequisitesMet(taskID string) (bool, error)
	MarkSessionDisconnected(sessionID string) error
}

type Launcher interface {
	LaunchTask(task Task) error
}

type Orchestrator struct {
	store    Store
	launcher Launcher
}

func New(store Store, launcher Launcher) *Orchestrator {
	return &Orchestrator{
		store:    store,
		launcher: launcher,
	}
}

func (o *Orchestrator) OnTaskCompleted(taskID string) ([]string, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task id required")
	}
	downstream, err := o.store.GetDownstreamTasks(taskID)
	if err != nil {
		return nil, err
	}

	launched := make([]string, 0, len(downstream))
	for _, next := range downstream {
		ready, err := o.store.AllPrerequisitesMet(next.ID)
		if err != nil {
			return launched, err
		}
		if !ready {
			continue
		}
		if err := o.launcher.LaunchTask(next); err != nil {
			return launched, err
		}
		launched = append(launched, next.ID)
	}
	return launched, nil
}

func (o *Orchestrator) OnHeartbeatTimeout(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	return o.store.MarkSessionDisconnected(sessionID)
}
