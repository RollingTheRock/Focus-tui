package agents

type Registry struct {
	sessions map[string]*Session
}

func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]*Session)}
}

func (r *Registry) Register(s *Session) {
	if s == nil {
		return
	}
	r.sessions[s.ID] = s
}

func (r *Registry) Remove(id string) {
	delete(r.sessions, id)
}

func (r *Registry) Clear() {
	r.sessions = make(map[string]*Session)
}

func (r *Registry) ByWorktree(worktreeID string) []*Session {
	var out []*Session
	for _, s := range r.sessions {
		if s.WorktreeID == worktreeID {
			out = append(out, s)
		}
	}
	return out
}

func (r *Registry) HasRunning(worktreeID string, provider Provider) bool {
	for _, s := range r.sessions {
		if s.WorktreeID == worktreeID && s.Provider == provider && s.State == SessionRunning {
			return true
		}
	}
	return false
}

func (r *Registry) All() []*Session {
	out := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		out = append(out, s)
	}
	return out
}
