package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const defaultSchemaDir = "schema"

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	yes := fs.Bool("yes", false, "Skip the wizard and accept default values")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane init [--yes]\n\n")
		fmt.Fprintf(os.Stderr, "Launch the interactive Lockplane project wizard. The wizard helps you\n")
		fmt.Fprintf(os.Stderr, "bootstrap a schema directory and will grow to cover more project setup\n")
		fmt.Fprintf(os.Stderr, "tasks over time. Use --yes to create the default schema/ directory\n")
		fmt.Fprintf(os.Stderr, "without any prompts.\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  lockplane init\n")
		fmt.Fprintf(os.Stderr, "  lockplane init --yes\n")
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	if *yes {
		if err := ensureSchemaDir(defaultSchemaDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "✓ Ready! Created %s/\n", defaultSchemaDir)
		return
	}

	if err := startInitWizard(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func ensureSchemaDir(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultSchemaDir
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("%s exists but is not a directory", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

func startInitWizard() error {
	model := newInitWizardModel()
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return err
	}
	if model.err != nil {
		return model.err
	}
	return nil
}

type initWizardModel struct {
	input         textinput.Model
	spinner       spinner.Model
	creating      bool
	done          bool
	err           error
	status        string
	schemaDir     string
	shouldQuit    bool
	width, height int
}

func newInitWizardModel() *initWizardModel {
	ti := textinput.New()
	ti.Placeholder = defaultSchemaDir
	ti.SetValue(defaultSchemaDir)
	ti.Focus()
	ti.CharLimit = 128

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &initWizardModel{
		input:     ti,
		spinner:   sp,
		status:    "",
		schemaDir: defaultSchemaDir,
	}
}

func (m *initWizardModel) Init() tea.Cmd {
	return textinput.Blink
}

type schemaCreatedMsg struct {
	Path string
}

type schemaErrorMsg struct {
	Err error
}

func createSchemaDirCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if err := ensureSchemaDir(path); err != nil {
			return schemaErrorMsg{Err: err}
		}
		return schemaCreatedMsg{Path: path}
	}
}

func (m *initWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.shouldQuit = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.creating || m.done {
				return m, nil
			}
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				value = defaultSchemaDir
			}
			m.schemaDir = value
			m.creating = true
			m.status = ""
			cmds := []tea.Cmd{
				createSchemaDirCmd(value),
				m.spinner.Tick,
			}
			return m, tea.Batch(cmds...)
		}
	case schemaCreatedMsg:
		m.creating = false
		m.done = true
		m.status = fmt.Sprintf("✓ Ready! Created %s/", msg.Path)
		m.shouldQuit = true
		return m, tea.Sequence(tea.Quit)
	case schemaErrorMsg:
		m.creating = false
		m.err = msg.Err
		m.status = fmt.Sprintf("Error: %v", msg.Err)
	case spinner.TickMsg:
		if m.creating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if !m.creating && !m.done {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *initWizardModel) View() string {
	var b strings.Builder

	b.WriteString("\n  Lockplane Init Wizard\n\n")
	b.WriteString("  This wizard bootstraps your project by creating a schema directory.\n")
	b.WriteString("  More setup steps are coming in future releases.\n\n")

	if m.creating {
		b.WriteString(fmt.Sprintf("  %s Creating %s/\n\n", m.spinner.View(), filepath.ToSlash(m.schemaDir)))
	} else if m.done {
		b.WriteString(fmt.Sprintf("  %s\n\n", m.status))
	} else {
		b.WriteString("  Where should we create your schema directory?\n")
		b.WriteString("  Press Enter to accept the default.\n\n")
		b.WriteString("  ")
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		b.WriteString("  ↑/↓ Move | Enter Confirm | Esc Cancel\n\n")
	}

	if m.err != nil && !m.creating && !m.done {
		b.WriteString(fmt.Sprintf("  Error: %v\n\n", m.err))
	}

	if m.done {
		b.WriteString("  See you soon! 👋\n\n")
	}

	return b.String()
}
