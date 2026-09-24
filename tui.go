package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int8

const (
	ViewText ViewState = iota
	ViewData
)

type UIModel struct {
	stream     []InstructionItem
	cursor     int
	history    []int
	height     int
	width      int
	format     string
	viewState  ViewState
	dataSecs   []DataSection
	dataCursor int
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
		case "tab":
			if m.viewState == ViewText {
				m.viewState = ViewData
			} else {
				m.viewState = ViewText
			}
		case "up", "k":
			if m.viewState == ViewText && m.cursor > 0 {
				m.cursor--
			} else if m.viewState == ViewData && m.dataCursor > 0 {
				m.dataCursor--
			}
		case "down", "j":
			if m.viewState == ViewText && m.cursor < len(m.stream)-1 {
				m.cursor++
			} else if m.viewState == ViewData && len(m.dataSecs) > 0 {
				maxLines := (len(m.dataSecs[0].Data) + 15) / 16
				if m.dataCursor < maxLines-1 {
					m.dataCursor++
				}
			}
		case "pgup":
			if m.viewState == ViewText {
				m.cursor -= m.height / 2
				if m.cursor < 0 {
					m.cursor = 0
				}
			} else if m.viewState == ViewData {
				m.dataCursor -= m.height / 2
				if m.dataCursor < 0 {
					m.dataCursor = 0
				}
			}
		case "pgdown":
			if m.viewState == ViewText {
				m.cursor += m.height / 2
				if m.cursor >= len(m.stream) {
					m.cursor = len(m.stream) - 1
				}
			} else if m.viewState == ViewData && len(m.dataSecs) > 0 {
				m.dataCursor += m.height / 2
				maxLines := (len(m.dataSecs[0].Data) + 15) / 16
				if m.dataCursor >= maxLines {
					m.dataCursor = maxLines - 1
				}
			}
		case "enter":
			if m.viewState == ViewText {
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
			}
		case "esc", "backspace":
			if m.viewState == ViewText && len(m.history) > 0 {
				m.cursor = m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
			}
		}
	}
	return m, nil
}

func (m UIModel) renderDataView() string {
	if len(m.dataSecs) == 0 {
		return "No data sections available in this binary.\n"
	}

	sec := m.dataSecs[0]
	var b strings.Builder

	b.WriteString(fmt.Sprintf("--- Section: %s ---\n", sec.Name))

	bytesPerLine := 16
	totalLines := (len(sec.Data) + bytesPerLine - 1) / bytesPerLine

	viewRange := m.height - 5
	if viewRange < 1 {
		viewRange = 1
	}

	start := m.dataCursor - (viewRange / 2)
	if start < 0 {
		start = 0
	}
	end := start + viewRange
	if end > totalLines {
		end = totalLines
	}

	for i := start; i < end; i++ {
		offset := i * bytesPerLine
		chunkEnd := offset + bytesPerLine
		if chunkEnd > len(sec.Data) {
			chunkEnd = len(sec.Data)
		}
		chunk := sec.Data[offset:chunkEnd]

		b.WriteString(fmt.Sprintf("%08x  ", sec.Address+uint64(offset)))

		var asciiStr strings.Builder
		for j := 0; j < bytesPerLine; j++ {
			if j < len(chunk) {
				b.WriteString(fmt.Sprintf("%02x ", chunk[j]))
				if chunk[j] >= 32 && chunk[j] <= 126 {
					asciiStr.WriteByte(chunk[j])
				} else {
					asciiStr.WriteByte('.')
				}
			} else {
				b.WriteString("   ")
				asciiStr.WriteByte(' ')
			}

			if j == 7 {
				b.WriteString(" ")
			}
		}

		b.WriteString(fmt.Sprintf(" |%s|\n", asciiStr.String()))
	}

	return b.String()
}

func (m UIModel) View() string {
	var b strings.Builder

	header := "| [Tab] Swap View  [j/k/pg] Move  [Enter] Follow  [Esc] Back  [q] Quit "
	b.WriteString(fmt.Sprintf("%s %s\n%s\n", m.format, header, strings.Repeat("─", len(header)+len(m.format))))

	if m.viewState == ViewData {
		b.WriteString(m.renderDataView())
		return b.String()
	}

	// Calculate sliding window for Text View
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
			targetHint = " ↵"
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
