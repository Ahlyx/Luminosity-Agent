package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// ── palette ───────────────────────────────────────────────────────────────────
var (
	gold    = lipgloss.Color("#FFD600")
	skyBlue = lipgloss.Color("#7BB8E8")
	dimBlue = lipgloss.Color("#2E6FB5")
	sand    = lipgloss.Color("#C8A96E")
	dimText = lipgloss.Color("#555566")
	white   = lipgloss.Color("#EEEEEE")
	red     = lipgloss.Color("#FF5F57")
	green   = lipgloss.Color("#28C840")
	amber   = lipgloss.Color("#FEBC2E")
)

// ── styles ────────────────────────────────────────────────────────────────────
var (
	outerBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(gold).
		Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
		Foreground(gold).
		Bold(true).
		Align(lipgloss.Center)

	subheaderStyle = lipgloss.NewStyle().
		Foreground(skyBlue).
		Align(lipgloss.Center)

	linkStyle = lipgloss.NewStyle().
		Foreground(skyBlue).
		Underline(true)

	sunStyle = lipgloss.NewStyle().
		Foreground(gold)

	statusBarStyle = lipgloss.NewStyle().
		Background(dimBlue).
		Foreground(white).
		Padding(0, 1)

	userMsgStyle = lipgloss.NewStyle().
		Foreground(gold).
		Bold(true)

	assistantMsgStyle = lipgloss.NewStyle().
		Foreground(white)

	toolMsgStyle = lipgloss.NewStyle().
		Foreground(dimText).
		Italic(true)

	errorStyle = lipgloss.NewStyle().
		Foreground(red)

	inputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dimBlue).
		Padding(0, 1)

	inputFocusStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(gold).
		Padding(0, 1)

	dotRed   = lipgloss.NewStyle().Foreground(red).Render("●")
	dotAmber = lipgloss.NewStyle().Foreground(amber).Render("●")
	dotGreen = lipgloss.NewStyle().Foreground(green).Render("●")
)

// ── splash banner ─────────────────────────────────────────────────────────────
const sunArt = `
    \  ·  /
  ·  \ | /  ·
─ ──(  ✦  )── ─
  ·  / | \  ·
    /  ·  \
`

func RenderBanner(width int) string {
	sun := sunStyle.Render(sunArt)
	title := headerStyle.Width(width).Render("✦  LUMINOSITY AGENT  ✦")
	sub := subheaderStyle.Width(width).Render("local ai assistant · small model optimized")

	links := lipgloss.JoinHorizontal(
		lipgloss.Center,
		linkStyle.Render("github.com/ahlyx"),
		lipgloss.NewStyle().Foreground(dimText).Render("  ·  "),
		linkStyle.Render("@AhIyxx"),
	)
	linksRow := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(links)

	help := lipgloss.NewStyle().
		Foreground(dimText).
		Width(width).
		Align(lipgloss.Center).
		Render("/help · /memory · /remember · /tools · /quit")

	banner := lipgloss.JoinVertical(
		lipgloss.Center,
		sun,
		title,
		sub,
		"",
		linksRow,
		help,
	)

	return outerBorder.Width(width - 4).Render(banner)
}

// ── message types ─────────────────────────────────────────────────────────────
type MsgKind int

const (
	KindUser MsgKind = iota
	KindAssistant
	KindTool
	KindError
	KindSystem
	KindThinkingStart
	KindThinkingStop
)

type AgentMsg struct {
	Kind MsgKind
	Text string
}

type SubmitMsg struct {
	Input string
}

type Message struct {
	Kind    MsgKind
	Content string
}

func (m Message) Render(width int) string {
	switch m.Kind {
	case KindUser:
		prefix := userMsgStyle.Render("▸ you  ")
		body := lipgloss.NewStyle().Foreground(gold).Width(width - 8).Render(m.Content)
		return lipgloss.JoinHorizontal(lipgloss.Top, prefix, body)
	case KindAssistant:
		prefix := lipgloss.NewStyle().Foreground(skyBlue).Bold(true).Render("✦ lumi ")
		body := assistantMsgStyle.Width(width - 8).Render(m.Content)
		return lipgloss.JoinHorizontal(lipgloss.Top, prefix, body)
	case KindTool:
		return toolMsgStyle.Width(width).Render("  ⚙  " + m.Content)
	case KindError:
		return errorStyle.Width(width).Render("  ✗  " + m.Content)
	case KindSystem:
		return lipgloss.NewStyle().Foreground(dimText).Width(width).Align(lipgloss.Center).Render(m.Content)
	}
	return m.Content
}

// ── model ─────────────────────────────────────────────────────────────────────
type Model struct {
	width    int
	height   int
	viewport viewport.Model
	input    textarea.Model
	messages []Message
	thinking bool
	ready    bool
	inputCh  chan<- string
}

func New(inputCh chan<- string) Model {
	ta := textarea.New()
	ta.Placeholder = "type a message or /help..."
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return Model{
		input:   ta,
		inputCh: inputCh,
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// renderLines builds viewport content from current state — pure function on value
func (m Model) renderLines() string {
	var lines []string
	if len(m.messages) == 0 {
		lines = append(lines, RenderBanner(m.width-4))
	}
	for _, msg := range m.messages {
		lines = append(lines, msg.Render(m.width-4))
		lines = append(lines, "")
	}
	if m.thinking {
		lines = append(lines, lipgloss.NewStyle().Foreground(gold).Render("  ✦  thinking..."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		inputH := 5
		statusH := 1
		vpHeight := m.height - inputH - statusH - 4

		if !m.ready {
			m.viewport = viewport.New(m.width-4, vpHeight)
			m.viewport.SetContent(RenderBanner(m.width - 4))
			m.ready = true
		} else {
			m.viewport.Width = m.width - 4
			m.viewport.Height = vpHeight
			m.viewport.SetContent(m.renderLines())
		}
		m.input.SetWidth(m.width - 6)

	case AgentMsg:
		switch msg.Kind {
		case KindThinkingStart:
			m.thinking = true
		case KindThinkingStop:
			m.thinking = false
		default:
			kindMap := map[MsgKind]MsgKind{
				KindAssistant: KindAssistant,
				KindTool:      KindTool,
				KindError:     KindError,
				KindSystem:    KindSystem,
			}
			if k, ok := kindMap[msg.Kind]; ok {
				m.messages = append(m.messages, Message{Kind: k, Content: msg.Text})
			}
		}
		if m.ready {
			m.viewport.SetContent(m.renderLines())
			m.viewport.GotoBottom()
		}

	case SubmitMsg:
		if m.inputCh != nil {
			select {
			case m.inputCh <- msg.Input:
			default:
			}
		}
		m.thinking = true
		if m.ready {
			m.viewport.SetContent(m.renderLines())
			m.viewport.GotoBottom()
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				break
			}
			m.input.Reset()
			m.messages = append(m.messages, Message{Kind: KindUser, Content: val})
			if m.ready {
				m.viewport.SetContent(m.renderLines())
				m.viewport.GotoBottom()
			}
			return m, func() tea.Msg { return SubmitMsg{Input: val} }
		}
	}

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "initializing..."
	}

	dots := dotRed + " " + dotAmber + " " + dotGreen
	modelLabel := lipgloss.NewStyle().Foreground(skyBlue).Render("qwen3.5-4b")
	ctxLabel := lipgloss.NewStyle().Foreground(sand).Render("8192 ctx")
	statusRight := modelLabel + "  " + ctxLabel
	statusLeft := dots + "  luminosity"
	gap := m.width - lipgloss.Width(statusLeft) - lipgloss.Width(statusRight) - 4
	if gap < 0 {
		gap = 0
	}
	statusBar := statusBarStyle.Width(m.width).Render(
		statusLeft + strings.Repeat(" ", gap) + statusRight,
	)

	borderStyle := inputStyle
	if m.input.Focused() {
		borderStyle = inputFocusStyle
	}
	inputBox := borderStyle.Width(m.width - 4).Render(m.input.View())

	vpH := m.height - lipgloss.Height(statusBar) - lipgloss.Height(inputBox) - 4

	return lipgloss.JoinVertical(
		lipgloss.Left,
		statusBar,
		outerBorder.Width(m.width-4).Height(vpH).Render(m.viewport.View()),
		inputBox,
	)
}