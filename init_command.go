package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultSchemaDir         = "schema"
	lockplaneConfigFilename  = "lockplane.toml"
	defaultLockplaneTomlBody = `default_environment = "local"

[environments.local]
description = "Local development database"
schema_path = "."
database_url = "postgresql://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
shadow_database_url = "postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable"
`
)

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	yes := fs.Bool("yes", false, "Skip the wizard and accept default values")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lockplane init [--yes]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Launch the interactive Lockplane project wizard. The wizard bootstraps\n")
		_, _ = fmt.Fprintf(os.Stderr, "the schema/ directory and creates schema/lockplane.toml.\n")
		_, _ = fmt.Fprintf(os.Stderr, "Use --yes to accept defaults without prompts.\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nExamples:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init --yes\n")
	}

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	if *yes {
		result, err := bootstrapSchemaDirectory()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		reportBootstrapResult(os.Stdout, result)
		return
	}

	if err := startInitWizard(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type bootstrapResult struct {
	SchemaDir        string
	ConfigPath       string
	SchemaDirCreated bool
	ConfigCreated    bool
}

func ensureSchemaDir(path string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultSchemaDir
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, nil
		}
		return false, fmt.Errorf("%s exists but is not a directory", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, os.MkdirAll(path, 0o755)
}

func bootstrapSchemaDirectory() (*bootstrapResult, error) {
	dirCreated, err := ensureSchemaDir(defaultSchemaDir)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(defaultSchemaDir, lockplaneConfigFilename)
	if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
		return nil, fmt.Errorf("%s already exists.\n\nEdit the existing file or delete it if you want to re-initialize.", filepath.ToSlash(configPath))
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := os.WriteFile(configPath, []byte(defaultLockplaneTomlBody), 0o644); err != nil {
		return nil, err
	}

	return &bootstrapResult{
		SchemaDir:        defaultSchemaDir,
		ConfigPath:       configPath,
		SchemaDirCreated: dirCreated,
		ConfigCreated:    true,
	}, nil
}

func reportBootstrapResult(out *os.File, result *bootstrapResult) {
	if result == nil {
		return
	}

	if result.SchemaDirCreated {
		_, _ = fmt.Fprintf(out, "✓ Created %s/\n", filepath.ToSlash(result.SchemaDir))
	} else {
		_, _ = fmt.Fprintf(out, "• Using existing %s/\n", filepath.ToSlash(result.SchemaDir))
	}

	if result.ConfigCreated {
		_, _ = fmt.Fprintf(out, "✓ Wrote %s\n", filepath.ToSlash(result.ConfigPath))
	}
}

func startInitWizard() error {
	model := newInitWizardModel()
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return err
	}
	if model.err != nil {
		return model.err
	}
	reportBootstrapResult(os.Stdout, model.result)
	return nil
}

type initWizardModel struct {
	spinner    spinner.Model
	creating   bool
	done       bool
	err        error
	status     string
	result     *bootstrapResult
	shouldQuit bool
}

func newInitWizardModel() *initWizardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &initWizardModel{
		spinner: sp,
	}
}

func (m *initWizardModel) Init() tea.Cmd {
	return nil
}

type bootstrapResultMsg struct {
	Result *bootstrapResult
}

type bootstrapErrorMsg struct {
	Err error
}

func createBootstrapCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := bootstrapSchemaDirectory()
		if err != nil {
			return bootstrapErrorMsg{Err: err}
		}
		return bootstrapResultMsg{Result: result}
	}
}

func (m *initWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.shouldQuit = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.creating || m.done {
				return m, nil
			}
			m.creating = true
			m.status = ""
			return m, tea.Batch(createBootstrapCmd(), m.spinner.Tick)
		}
	case spinner.TickMsg:
		if m.creating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case bootstrapResultMsg:
		m.creating = false
		m.done = true
		m.result = msg.Result
		if msg.Result != nil {
			if msg.Result.SchemaDirCreated {
				m.status = fmt.Sprintf("✓ Ready! Created %s/ and wrote %s", filepath.ToSlash(msg.Result.SchemaDir), filepath.ToSlash(msg.Result.ConfigPath))
			} else {
				m.status = fmt.Sprintf("✓ Ready! Wrote %s", filepath.ToSlash(msg.Result.ConfigPath))
			}
		} else {
			m.status = "✓ Ready!"
		}
		m.shouldQuit = true
		return m, tea.Sequence(tea.Quit)
	case bootstrapErrorMsg:
		m.creating = false
		m.err = msg.Err
		m.status = fmt.Sprintf("Error: %v", msg.Err)
		return m, nil
	}

	return m, nil
}

func (m *initWizardModel) View() string {
	var b strings.Builder

	b.WriteString("\n  Lockplane Init Wizard\n\n")
	b.WriteString("  This wizard will create:\n")
	b.WriteString("    • schema/\n")
	b.WriteString("    • schema/lockplane.toml\n\n")

	if m.creating {
		b.WriteString(fmt.Sprintf("  %s Setting up schema/...\n\n", m.spinner.View()))
	} else if m.done {
		b.WriteString(fmt.Sprintf("  %s\n\n", m.status))
	} else {
		b.WriteString("  Press Enter to continue or Esc to cancel.\n\n")
	}

	if m.err != nil && !m.creating && !m.done {
		b.WriteString(fmt.Sprintf("  Error: %v\n\n", m.err))
	}

	if m.done {
		b.WriteString("  See you soon! 👋\n\n")
	}

	return b.String()
}
