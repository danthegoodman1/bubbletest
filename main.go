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
	l.Title = "Files"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	// Setup viewport
	vp := viewport.New(0, 0)
	vp.SetContent("Select a file to view its content")

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
	}
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

		if m.isFullscreen {
			// In fullscreen mode, the content pane takes the full screen
			m.viewport.Width = m.width
			m.viewport.Height = m.height
		} else {
			// Calculate pane widths for normal mode
			leftWidth := min(40, m.width/4)
			rightWidth := m.width - leftWidth - 4 // Account for borders

			// Update list size
			m.list.SetWidth(leftWidth - 2) // Account for border
			m.list.SetHeight(m.height - 2)

			// Update viewport size
			m.viewport.Width = rightWidth - 4 // Account for border (2px) + padding (2px)
			m.viewport.Height = m.height - 4  // Account for border (2px) + padding (2px)
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

	// Handle list navigation when navigator is focused
	if m.focusedPane == NavigatorPane {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	// Handle viewport navigation when content is focused
	if m.focusedPane == ContentPane {
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
	m.list.Select(0)
	m.viewport.SetContent("Select a file to view its content")
	m.fileContent = ""

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
		m.list.Select(0)
		m.viewport.SetContent("Select a file to view its content")
		m.fileContent = ""
	} else {
		// Read file content
		content, err := os.ReadFile(fileItem.path)
		if err != nil {
			m.fileContent = fmt.Sprintf("Error reading file: %v", err)
		} else {
			rawContent := string(content)
			if isMarkdownFile(fileItem.name) {
				// Render markdown with Glamour
				m.fileContent = renderMarkdown(rawContent, m.viewport.Width)
			} else {
				// Add line numbers for non-markdown files
				m.fileContent = addLineNumbers(rawContent)
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

	// Handle fullscreen mode for content pane
	if m.isFullscreen {
		return m.viewport.View()
	}

	// Calculate pane widths for normal mode
	leftWidth := min(40, m.width/4)
	rightWidth := m.width - leftWidth - 4 // Account for borders

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
		Height(m.height - 2).
		Render(m.list.View())

	rightPane := rightStyle.
		Width(rightWidth).
		Height(m.height - 2).
		Padding(1).
		Render(m.viewport.View())

	// Combine panes horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
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

// renderMarkdown renders markdown content using Glamour
func renderMarkdown(content string, width int) string {
	// Create a custom renderer for the terminal
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content // Fall back to raw content if rendering fails
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content // Fall back to raw content if rendering fails
	}

	return rendered
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
