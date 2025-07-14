package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Pane represents which pane is currently active or being hovered
type Pane int

const (
	NavigatorPane Pane = iota
	ContentPane
)

// Mode represents the current interaction mode
type Mode int

const (
	NavigatorMode     Mode = iota // Navigator pane is focused
	PaneSelectionMode             // In pane selection mode (can switch between panes)
)

// FileItem represents a file in the navigator
type FileItem struct {
	name  string
	path  string
	isDir bool
}

func (f FileItem) FilterValue() string { return f.name }
func (f FileItem) Title() string       { return f.name }
func (f FileItem) Description() string {
	if f.isDir {
		return "Directory"
	}
	return "File"
}

// Model holds the application state
type Model struct {
	list             list.Model
	viewport         viewport.Model
	mode             Mode
	selectedPane     Pane
	focusedPane      Pane
	width            int
	height           int
	currentDir       string
	directoryHistory []string
	fileContent      string
	isFullscreen     bool
	markdownRenderer *glamour.TermRenderer
	showLineNumbers  bool
	currentFilePath  string
}

// Initialize the model
func initialModel() Model {
	// Get current directory
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = "."
	}

	// Create file list
	files := getFileList(currentDir)

	// Setup list
	l := list.New(files, list.NewDefaultDelegate(), 0, 0)
	l.Title = formatDirectoryPath(currentDir)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	setListTitleStyle(&l)

	// Setup viewport
	vp := viewport.New(0, 0)
	vp.SetContent("Select a file to view its content")

	// Initialize markdown renderer
	markdownRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100), // Fixed reasonable width
	)
	if err != nil {
		markdownRenderer = nil // Fall back to no rendering if creation fails
	}

	return Model{
		list:             l,
		viewport:         vp,
		mode:             NavigatorMode,
		selectedPane:     NavigatorPane,
		focusedPane:      NavigatorPane,
		currentDir:       currentDir,
		directoryHistory: []string{},
		fileContent:      "",
		isFullscreen:     false,
		markdownRenderer: markdownRenderer,
		showLineNumbers:  true,
		currentFilePath:  "",
	}
}

// setListTitleStyle applies consistent styling to the list title
func setListTitleStyle(l *list.Model) {
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		MarginBottom(1)
}

// formatDirectoryPath formats a directory path with ~ substitution for home directory
func formatDirectoryPath(path string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if strings.HasPrefix(path, homeDir) {
		return strings.Replace(path, homeDir, "~", 1)
	}

	return path
}

// Get list of files in directory
func getFileList(dir string) []list.Item {
	var items []list.Item

	entries, err := os.ReadDir(dir)
	if err != nil {
		return items
	}

	// Add parent directory if not root
	if dir != "/" {
		items = append(items, FileItem{
			name:  "..",
			path:  filepath.Dir(dir),
			isDir: true,
		})
	}

	// Sort entries: directories first, then files
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		items = append(items, FileItem{
			name:  entry.Name(),
			path:  filepath.Join(dir, entry.Name()),
			isDir: entry.IsDir(),
		})
	}

	return items
}

// Styles for the application
var (
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("42")) // Green

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")) // White/Gray

	selectedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("51")) // Cyan
)

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		helpHeight := 2 // Help text line + padding line

		if m.isFullscreen {
			// In fullscreen mode, the content pane takes the full screen minus help
			m.viewport.Width = m.width
			m.viewport.Height = m.height - helpHeight
		} else {
			// Calculate pane widths for normal mode
			leftWidth := min(40, m.width/4)
			rightWidth := m.width - leftWidth - 4 // Account for borders
			paneHeight := m.height - helpHeight

			// Update list size
			m.list.SetWidth(leftWidth - 2) // Account for border
			m.list.SetHeight(paneHeight - 2)

			// Update viewport size
			m.viewport.Width = rightWidth - 4  // Account for border (2px) + padding (2px)
			m.viewport.Height = paneHeight - 4 // Account for border (2px) + padding (2px)
		}

		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case NavigatorMode:
			return m.handleNavigatorMode(msg)
		case PaneSelectionMode:
			return m.handlePaneSelectionMode(msg)
		}
	}

	return m, nil
}

func (m Model) handleNavigatorMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.isFullscreen {
			// Exit fullscreen mode
			m.isFullscreen = false
			return m, nil
		}
		// Enter pane selection mode
		m.mode = PaneSelectionMode
		m.selectedPane = m.focusedPane
		return m, nil
	case "left":
		// Switch focus to navigator pane when on content pane
		if m.focusedPane == ContentPane {
			m.focusedPane = NavigatorPane
			return m, nil
		}
	case "l":
		// Toggle line numbers when focused on content pane
		if m.focusedPane == ContentPane {
			m.showLineNumbers = !m.showLineNumbers
			return m.rerenderCurrentFile()
		}
		return m, nil
	case "f":
		// Toggle fullscreen only for content pane when focused
		if m.focusedPane == ContentPane {
			m.isFullscreen = !m.isFullscreen
		}
		return m, nil
	case "z":
		if m.focusedPane == NavigatorPane {
			return m.goToPreviousDirectory()
		}
		return m, nil
	case "enter":
		if m.focusedPane == NavigatorPane {
			return m.handleFileSelection()
		}
		return m, nil
	}

	// Handle navigation based on focused pane
	switch m.focusedPane {
	case NavigatorPane:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case ContentPane:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handlePaneSelectionMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = NavigatorMode
		return m, nil
	case "left":
		m.selectedPane = NavigatorPane
		return m, nil
	case "right":
		m.selectedPane = ContentPane
		return m, nil
	case "enter":
		m.focusedPane = m.selectedPane
		m.mode = NavigatorMode
		return m, nil
	}

	return m, nil
}

func (m Model) goToPreviousDirectory() (tea.Model, tea.Cmd) {
	// Check if there's a previous directory
	if len(m.directoryHistory) == 0 {
		return m, nil
	}

	// Pop the last directory from history
	prevDir := m.directoryHistory[len(m.directoryHistory)-1]
	m.directoryHistory = m.directoryHistory[:len(m.directoryHistory)-1]

	// Navigate to the previous directory
	m.currentDir = prevDir
	files := getFileList(m.currentDir)
	m.list.SetItems(files)
	m.list.Title = formatDirectoryPath(m.currentDir)
	setListTitleStyle(&m.list)
	m.list.Select(0)
	m.viewport.SetContent("Select a file to view its content")
	m.fileContent = ""

	return m, nil
}

func (m Model) rerenderCurrentFile() (tea.Model, tea.Cmd) {
	if m.currentFilePath == "" {
		return m, nil
	}

	// Read file content
	content, err := os.ReadFile(m.currentFilePath)
	if err != nil {
		m.fileContent = fmt.Sprintf("Error reading file: %v", err)
	} else {
		rawContent := string(content)
		filename := filepath.Base(m.currentFilePath)
		if isMarkdownFile(filename) {
			// Render markdown with Glamour (no line numbers)
			m.fileContent = m.renderMarkdown(rawContent)
		} else {
			// Add line numbers for non-markdown files if enabled
			if m.showLineNumbers {
				m.fileContent = addLineNumbers(rawContent)
			} else {
				m.fileContent = rawContent
			}
		}
	}
	m.viewport.SetContent(m.fileContent)

	return m, nil
}

func (m Model) handleFileSelection() (tea.Model, tea.Cmd) {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		return m, nil
	}

	fileItem := selectedItem.(FileItem)

	if fileItem.isDir {
		// Add current directory to history before changing
		m.directoryHistory = append(m.directoryHistory, m.currentDir)

		// Change directory
		m.currentDir = fileItem.path
		files := getFileList(m.currentDir)
		m.list.SetItems(files)
		m.list.Title = formatDirectoryPath(m.currentDir)
		setListTitleStyle(&m.list)
		m.list.Select(0)
		m.viewport.SetContent("Select a file to view its content")
		m.fileContent = ""
		m.currentFilePath = ""
	} else {
		// Store current file path
		m.currentFilePath = fileItem.path

		// Read file content
		content, err := os.ReadFile(fileItem.path)
		if err != nil {
			m.fileContent = fmt.Sprintf("Error reading file: %v", err)
		} else {
			rawContent := string(content)
			if isMarkdownFile(fileItem.name) {
				// Render markdown with Glamour (no line numbers)
				m.fileContent = m.renderMarkdown(rawContent)
			} else {
				// Add line numbers for non-markdown files if enabled
				if m.showLineNumbers {
					m.fileContent = addLineNumbers(rawContent)
				} else {
					m.fileContent = rawContent
				}
			}
		}
		m.focusedPane = ContentPane
		m.viewport.SetContent(m.fileContent)
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Generate help text
	helpText := m.getHelpText()
	helpHeight := 2 // Help text line + padding line

	// Handle fullscreen mode for content pane
	if m.isFullscreen {
		return helpText + "\n\n" + m.viewport.View()
	}

	// Calculate pane widths for normal mode
	leftWidth := min(40, m.width/4)
	rightWidth := m.width - leftWidth - 4 // Account for borders
	paneHeight := m.height - helpHeight

	// Style the left pane (navigator)
	var leftStyle lipgloss.Style
	if m.mode == PaneSelectionMode && m.selectedPane == NavigatorPane {
		leftStyle = selectedBorderStyle
	} else if m.mode == NavigatorMode && m.focusedPane == NavigatorPane {
		leftStyle = focusedBorderStyle
	} else {
		leftStyle = unfocusedBorderStyle
	}

	// Style the right pane (content)
	var rightStyle lipgloss.Style
	if m.mode == PaneSelectionMode && m.selectedPane == ContentPane {
		rightStyle = selectedBorderStyle
	} else if m.mode == NavigatorMode && m.focusedPane == ContentPane {
		rightStyle = focusedBorderStyle
	} else {
		rightStyle = unfocusedBorderStyle
	}

	// Create the panes
	leftPane := leftStyle.
		Width(leftWidth).
		Height(paneHeight - 2).
		Render(m.list.View())

	rightPane := rightStyle.
		Width(rightWidth).
		Height(paneHeight - 2).
		Padding(1).
		Render(m.viewport.View())

	// Combine panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Add help text at the top
	return helpText + "\n" + panes
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// addLineNumbers adds line numbers to the content
func addLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	// Calculate the width needed for line numbers
	maxLineNum := len(lines)
	lineNumWidth := len(strconv.Itoa(maxLineNum))

	var result strings.Builder
	for i, line := range lines {
		lineNum := i + 1
		// Format line number with padding
		lineNumStr := fmt.Sprintf("%*d", lineNumWidth, lineNum)
		result.WriteString(fmt.Sprintf("%s │ %s", lineNumStr, line))
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// isMarkdownFile checks if a file is a markdown file based on extension
func isMarkdownFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown"
}

// renderMarkdown renders markdown content using the pre-created Glamour renderer
func (m Model) renderMarkdown(content string) string {
	if m.markdownRenderer == nil {
		// Fall back to raw content if no renderer is available
		return content
	}

	rendered, err := m.markdownRenderer.Render(content)
	if err != nil {
		return content // Fall back to raw content if rendering fails
	}

	return rendered
}

func (m Model) getHelpText() string {
	// Style for highlighted keys
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).    // Black text
		Background(lipgloss.Color("#ccc")). // White background
		Padding(0, 1)                       // Small padding around text

	// Helper function to format key:action pairs
	formatHint := func(key, action string) string {
		return keyStyle.Render(key) + ":" + action
	}

	var hints []string

	// Common controls
	hints = append(hints, formatHint("q/ctrl+c", "quit"))

	if m.isFullscreen {
		// Fullscreen mode
		hints = append(hints, formatHint("f/esc", "exit fullscreen"))
		if m.focusedPane == ContentPane {
			hints = append(hints, formatHint("↑↓", "scroll"), formatHint("l", "toggle line numbers"))
		}
	} else if m.mode == PaneSelectionMode {
		// Pane selection mode
		hints = append(hints, formatHint("←→", "select pane"), formatHint("enter", "focus"), formatHint("esc", "back"))
	} else {
		// Normal mode
		hints = append(hints, formatHint("esc", "pane selection"))

		switch m.focusedPane {
		case NavigatorPane:
			hints = append(hints, formatHint("↑↓", "navigate"), formatHint("enter", "select"), formatHint("z", "back"))
		case ContentPane:
			hints = append(hints, formatHint("↑↓", "scroll"), formatHint("←", "back to navigator"), formatHint("l", "toggle line numbers"), formatHint("f", "fullscreen"))
		}
	}

	return " " + strings.Join(hints, "    ") // 4 spaces between items
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
