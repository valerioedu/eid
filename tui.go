package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewState int8

const (
	ViewText ViewState = iota
	ViewData
)

var (
	statusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Bold(true).
			Padding(0, 1)

	selectedLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(true)

	styleAddr     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleRaw      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleMnemonic = lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Bold(true)
	styleOperand  = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
	styleTarget   = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))

	styleDataHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true).Underline(true)
	styleHex        = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	styleAscii      = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
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
	symbols    map[uint64]string
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

	b.WriteString(styleDataHeader.Render(fmt.Sprintf("--- Section: %s ---", sec.Name)) + "\n\n")

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

		addrStr := fmt.Sprintf("%08x  ", sec.Address+uint64(offset))

		var hexStr strings.Builder
		var asciiStr strings.Builder
		for j := 0; j < bytesPerLine; j++ {
			if j < len(chunk) {
				hexStr.WriteString(fmt.Sprintf("%02x ", chunk[j]))
				if chunk[j] >= 32 && chunk[j] <= 126 {
					asciiStr.WriteByte(chunk[j])
				} else {
					asciiStr.WriteByte('.')
				}
			} else {
				hexStr.WriteString("   ")
				asciiStr.WriteByte(' ')
			}

			if j == 7 {
				hexStr.WriteString(" ")
			}
		}

		linePayload := fmt.Sprintf("%s%s |%s|", addrStr, hexStr.String(), asciiStr.String())

		if i == m.dataCursor {
			b.WriteString(selectedLine.Render(" ► "+linePayload) + "\n")
		} else {
			styledLine := fmt.Sprintf("   %s%s |%s|",
				styleAddr.Render(addrStr),
				styleHex.Render(hexStr.String()),
				styleAscii.Render(asciiStr.String()),
			)
			b.WriteString(styledLine + "\n")
		}
	}

	return b.String()
}

func (m UIModel) View() string {
	var b strings.Builder

	headerText := fmt.Sprintf("%s | [Tab] Swap View  [j/k/pg] Move  [Enter] Follow  [Esc] Back  [q] Quit", m.format)
	b.WriteString(statusBar.Width(m.width).Render(headerText) + "\n\n")

	if m.viewState == ViewData {
		b.WriteString(m.renderDataView())
		return b.String()
	}

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

		addr := fmt.Sprintf("0x%08x", item.Address)
		raw := fmt.Sprintf("%08x", item.Raw)
		mnemonic := fmt.Sprintf("%-8s", item.Decoded.Mnemonics)
		operands := fmt.Sprintf("%-20s", item.Decoded.Operands)

		targetHint := ""
		if item.Decoded.Target != 0 {
			if symName, exists := m.symbols[item.Decoded.Target]; exists {
				targetHint = fmt.Sprintf(" <%s> ↵", symName)
			} else {
				targetHint = " ↵"
			}
		}

		currentLabel := ""
		if symName, exists := m.symbols[item.Address]; exists {
			currentLabel = fmt.Sprintf(" <%s>", symName)
		}

		if i == m.cursor {
			selStr := fmt.Sprintf(" ► %s  %s  %s %s%s%s", addr, raw, mnemonic, operands, targetHint, currentLabel)
			b.WriteString(selectedLine.Render(selStr) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s  %s  %s %s%s%s\n",
				styleAddr.Render(addr),
				styleRaw.Render(raw),
				styleMnemonic.Render(mnemonic),
				styleOperand.Render(operands),
				styleTarget.Render(targetHint),
				styleTarget.Render(currentLabel),
			))
		}
	}

	return b.String()
}
