package app

import (
	"strings"

	"focus/internal/config"
	"focus/internal/models"
	"focus/internal/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type cityPickedMsg struct{ city string }
type closeCityPickerMsg struct{}

type cityPickerOverlay struct {
	zones   []zoneGroup
	zoneIdx int
	cityIdx int
	width   int
	height  int
}

type zoneGroup struct {
	offset string
	cities []string
}

var cityPickerZones = []zoneGroup{
	{offset: "UTC-08", cities: []string{"Los Angeles", "San Francisco", "Vancouver"}},
	{offset: "UTC-05", cities: []string{"New York", "Toronto"}},
	{offset: "UTC+00", cities: []string{"London", "Lisbon"}},
	{offset: "UTC+01", cities: []string{"Paris", "Berlin"}},
	{offset: "UTC+04", cities: []string{"Dubai"}},
	{offset: "UTC+08", cities: []string{"Beijing", "Shanghai", "Hangzhou", "Nanjing", "Wuhan", "Changsha", "Zhengzhou", "Hong Kong", "Shenzhen", "Chengdu", "Xi'an", "Singapore"}},
	{offset: "UTC+09", cities: []string{"Tokyo", "Seoul"}},
	{offset: "UTC+11", cities: []string{"Sydney"}},
}

func newCityPickerOverlay(cfg config.Config) *cityPickerOverlay {
	p := &cityPickerOverlay{zones: cityPickerZones}
	for zi, z := range p.zones {
		for ci, c := range z.cities {
			if c == cfg.Weather.City {
				p.zoneIdx = zi
				p.cityIdx = ci
				return p
			}
		}
	}
	return p
}

func (p *cityPickerOverlay) Init() tea.Cmd { return nil }

func (p *cityPickerOverlay) Update(msg tea.Msg) (models.Panel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if p.zoneIdx > 0 {
				p.zoneIdx--
				p.cityIdx = 0
			}
		case "right", "l":
			if p.zoneIdx < len(p.zones)-1 {
				p.zoneIdx++
				p.cityIdx = 0
			}
		case "up", "k":
			if p.cityIdx > 0 {
				p.cityIdx--
			}
		case "down", "j":
			cities := p.zones[p.zoneIdx].cities
			if p.cityIdx < len(cities)-1 {
				p.cityIdx++
			}
		case "enter":
			return p, func() tea.Msg {
				return cityPickedMsg{city: p.zones[p.zoneIdx].cities[p.cityIdx]}
			}
		case "esc", "q":
			return p, func() tea.Msg { return closeCityPickerMsg{} }
		}
	}
	return p, nil
}

func (p *cityPickerOverlay) View() string {
	w := p.width
	if w <= 0 {
		w = 40
	}
	h := p.height
	if h <= 0 {
		h = 15
	}

	accent := lipgloss.NewStyle().Foreground(styles.Accent).Bold(true)
	text := lipgloss.NewStyle().Foreground(styles.Text)
	subtle := lipgloss.NewStyle().Foreground(styles.Subtle)

	var b strings.Builder

	// Timezone bar.
	var zparts []string
	for i, z := range p.zones {
		off := strings.TrimPrefix(z.offset, "UTC")
		if i == p.zoneIdx {
			zparts = append(zparts, accent.Render("["+off+"]"))
		} else {
			zparts = append(zparts, subtle.Render(off))
		}
	}
	b.WriteString("  " + strings.Join(zparts, "  ") + "\n")
	b.WriteString("  " + strings.Repeat("─", w-4) + "\n")

	// City list.
	z := p.zones[p.zoneIdx]
	visibleCount := h - 5
	if visibleCount < 3 {
		visibleCount = 3
	}
	start := p.cityIdx - visibleCount/2
	if start < 0 {
		start = 0
	}
	end := start + visibleCount
	if end > len(z.cities) {
		end = len(z.cities)
		start = end - visibleCount
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		if i == p.cityIdx {
			b.WriteString(accent.Render("> "+z.cities[i]) + "\n")
		} else {
			b.WriteString(text.Render("  "+z.cities[i]) + "\n")
		}
	}

	b.WriteString(subtle.Render("\n  [←→] zone  [↑↓] city  [enter] select  [esc] cancel"))
	return b.String()
}

func (p *cityPickerOverlay) SetSize(width, height int) {
	p.width = width
	p.height = height
}
