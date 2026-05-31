package styles

import (
	"sync"

	"charm.land/lipgloss/v2"
)

var StyleCache = &styleCache{
	maxWidth: make(map[int]lipgloss.Style),
	width:    make(map[int]lipgloss.Style),
}

type styleCache struct {
	mu       sync.RWMutex
	maxWidth map[int]lipgloss.Style
	width    map[int]lipgloss.Style
}

func (c *styleCache) MaxWidth(w int) lipgloss.Style {
	c.mu.RLock()
	if s, ok := c.maxWidth[w]; ok {
		c.mu.RUnlock()
		return s
	}
	c.mu.RUnlock()

	s := lipgloss.NewStyle().MaxWidth(w)
	c.mu.Lock()
	c.maxWidth[w] = s
	c.mu.Unlock()
	return s
}

func (c *styleCache) Width(w int) lipgloss.Style {
	c.mu.RLock()
	if s, ok := c.width[w]; ok {
		c.mu.RUnlock()
		return s
	}
	c.mu.RUnlock()

	s := lipgloss.NewStyle().Width(w)
	c.mu.Lock()
	c.width[w] = s
	c.mu.Unlock()
	return s
}
