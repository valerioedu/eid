package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type UIModel struct {
	stream  []InstructionItem
	cursor  int
	history []int // Stack for tracking jumps
	height  int
	width   int
}

func (m UIModel) Init() tea.Cmd {
	return nil
}

func (m UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.stream)-1 {
				m.cursor++
			}
		case "pgup":
			m.cursor -= m.height / 2
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown":
			m.cursor += m.height / 2
			if m.cursor >= len(m.stream) {
				m.cursor = len(m.stream) - 1
			}
		case "enter":
			// Jump to target if the instruction is a branch
			target := m.stream[m.cursor].Decoded.Target
			if target != 0 {
				for idx, item := range m.stream {
					if item.Address == target {
						m.history = append(m.history, m.cursor)
						m.cursor = idx
						break
					}
				}
			}
		case "esc", "backspace":
			// Pop branch history
			if len(m.history) > 0 {
				m.cursor = m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
			}
		}
	}
	return m, nil
}

func (m UIModel) View() string {
	var b strings.Builder

	header := "| [j/k/pgup/pgdn] Move  [Enter] Follow  [Esc] Back  [q] Quit "
	b.WriteString(fmt.Sprintf("%s %s\n%s\n", getFileFormat(), header, strings.Repeat("─", len(header))))

	// Calculate sliding window
	viewRange := m.height - 4
	if viewRange < 1 {
		viewRange = 1
	}

	start := m.cursor - (viewRange / 2)
	if start < 0 {
		start = 0
	}
	end := start + viewRange
	if end > len(m.stream) {
		end = len(m.stream)
	}

	for i := start; i < end; i++ {
		item := m.stream[i]

		marker := "  "
		if i == m.cursor {
			marker = "► "
		}

		targetHint := ""
		if item.Decoded.Target != 0 {
			targetHint = " ↵" // Indicate branch is followable
		}

		b.WriteString(fmt.Sprintf("%s0x%08x  %08x  %-8s %-20s%s\n",
			marker,
			item.Address,
			item.Raw,
			item.Decoded.Mnemonics,
			item.Decoded.Operands,
			targetHint,
		))
	}

	return b.String()
}
