package analyze

import (
	"context"
	"errors"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type ConsolePresenter struct {
	config      Config
	programOpts []tea.ProgramOption
}

func NewConsolePresenter(config Config, programOpts ...tea.ProgramOption) *ConsolePresenter {
	return &ConsolePresenter{config: config, programOpts: programOpts}
}

func (p *ConsolePresenter) Run(ctx context.Context, state *StateStore, worker func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	model := NewBoundModel(p.config, state)
	model.SetCancel(cancel)
	program := p.startProgram(runCtx, model)
	snapshots, unsubscribe := state.Subscribe(16)
	defer unsubscribe()

	errCh := make(chan error, 1)

	go func() {
		for {
			select {
			case <-runCtx.Done():
				return
			case snapshot, ok := <-snapshots:
				if !ok {
					return
				}
				program.Send(snapshotMsg(snapshot))
			}
		}
	}()

	go func() {
		err := worker(runCtx)
		if err == nil {
			state.Update(markSessionComplete)
		}
		errCh <- err
		program.Quit()
	}()

	_, runErr := program.Run()
	if model.QuitRequested() {
		cancel()
		return nil
	}
	if runErr != nil {
		if errors.Is(runErr, tea.ErrProgramKilled) && errors.Is(runCtx.Err(), context.Canceled) {
			return nil
		}
		cancel()
		return runErr
	}
	if errors.Is(runCtx.Err(), context.Canceled) {
		return nil
	}

	cancel()
	workerErr := <-errCh
	if errors.Is(workerErr, context.Canceled) {
		return nil
	}
	return workerErr
}

func (p *ConsolePresenter) startProgram(ctx context.Context, model *Model) *tea.Program {
	options := []tea.ProgramOption{tea.WithContext(ctx)}
	if len(p.programOpts) > 0 {
		options = append(options, p.programOpts...)
	} else if isInteractiveSession() {
		options = append(options,
			tea.WithAltScreen(),
			tea.WithInput(os.Stdin),
			tea.WithOutput(os.Stdout),
		)
	} else {
		options = append(options, tea.WithInput(nil), tea.WithOutput(io.Discard))
	}

	return tea.NewProgram(model, options...)
}

func isInteractiveSession() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
