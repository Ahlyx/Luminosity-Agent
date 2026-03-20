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
 
	toolCallBlockStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(dimBlue).
				Foreground(amber).
				Padding(0, 1).
				MarginLeft(7)
 
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
 
// ── XML helpers ───────────────────────────────────────────────────────────────
// Duplicated from executor to avoid import cycle between tui and tools packages.
 
func xmlTagTUI(s, tag string) (string, bool) {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(s, open)
	if start == -1 {
		return "", false
	}
	start += len(open)
	end := strings.Index(s[start:], close)
	if end == -1 {
		return "", false
	}
	return strings.TrimSpace(s[start : start+end]), true
}
 
func hasToolCall(content string) bool {
	_, ok := xmlTagTUI(content, "tool")
	return ok
}
 
// renderToolCallBlock splits content into prose above the tool call and a
// styled tool block. Returns (prose, styledBlock).
func renderToolCallBlock(content string) (string, string) {
	toolName, _ := xmlTagTUI(content, "tool")
 
	// Everything before <tool> is prose
	toolStart := strings.Index(content, "<tool>")
	prose := ""
	if toolStart > 0 {
		prose = strings.TrimSpace(content[:toolStart])
	}
 
	// First non-empty parameter value becomes the subtitle
	paramKeys := []string{"url", "query", "path", "command", "name"}
	paramDisplay := ""
	for _, key := range paramKeys {
		if val, ok := xmlTagTUI(content, key); ok && val != "" {
			if len(val) > 55 {
				val = val[:55] + "…"
			}
			paramDisplay = lipgloss.NewStyle().
				Foreground(dimText).
				Render(key + ": " + val)
			break
		}
	}
 
	label := lipgloss.NewStyle().Foreground(amber).Bold(true).Render("⚙ " + toolName)
	var blockContent string
	if paramDisplay != "" {
		blockContent = label + "  " + paramDisplay
	} else {
		blockContent = label
	}
 
	return prose, toolCallBlockStyle.Render(blockContent)
}
 
// ── message types ─────────────────────────────────────────────────────────────
type MsgKind int
 
const (
	KindUser MsgKind = iota
	KindAssistant
	KindAssistantStart
	KindToken
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
 
		if hasToolCall(m.Content) {
			prose, toolBlock := renderToolCallBlock(m.Content)
			var parts []string
			if prose != "" {
				proseRendered := assistantMsgStyle.Width(width - 8).Render(prose)
				parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, prefix, proseRendered))
			} else {
				// No prose — still show the lumi prefix on its own line
				parts = append(parts, lipgloss.NewStyle().Foreground(skyBlue).Bold(true).Render("✦ lumi"))
			}
			parts = append(parts, toolBlock)
			return strings.Join(parts, "\n")
		}
 
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
 
func (m *Model) lastAssistantIdx() int {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Kind == KindAssistant {
			return i
		}
	}
	return -1
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
 
		case KindAssistantStart:
			m.thinking = false
			m.messages = append(m.messages, Message{Kind: KindAssistant, Content: ""})
 
		case KindToken:
			idx := m.lastAssistantIdx()
			if idx == -1 {
				m.messages = append(m.messages, Message{Kind: KindAssistant, Content: msg.Text})
			} else {
				m.messages[idx].Content += msg.Text
			}
			if m.ready {
				m.viewport.SetContent(m.renderLines())
				m.viewport.GotoBottom()
			}
			return m, tea.Batch(cmds...)
 
		case KindThinkingStart:
			m.thinking = true
 
		case KindThinkingStop:
			m.thinking = false
 
		case KindAssistant:
			m.messages = append(m.messages, Message{Kind: KindAssistant, Content: msg.Text})
 
		case KindTool:
			m.messages = append(m.messages, Message{Kind: KindTool, Content: msg.Text})
 
		case KindError:
			m.messages = append(m.messages, Message{Kind: KindError, Content: msg.Text})
 
		case KindSystem:
			m.messages = append(m.messages, Message{Kind: KindSystem, Content: msg.Text})
		}
 
		if m.ready {
			m.viewport.SetContent(m.renderLines())
			m.viewport.GotoBottom()
		}
 
	case SubmitMsg:
		inputCh := m.inputCh
		input := msg.Input
		go func() {
			if inputCh != nil {
				inputCh <- input
			}
		}()
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
	modelLabel := lipgloss.NewStyle().Foreground(skyBlue).Render("lumi · qwen3.5-4b")
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
