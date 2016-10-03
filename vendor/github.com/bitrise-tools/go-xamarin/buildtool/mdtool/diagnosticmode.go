package mdtool

import (
	"bufio"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/log"
)

// TimeoutHandlerModel ....
type TimeoutHandlerModel struct {
	timer   *time.Timer
	timeout time.Duration

	running bool

	onTimeout func()
}

// Start ...
func (handler *TimeoutHandlerModel) Start() {
	if &handler.timeout != nil {
		handler.timer = time.NewTimer(handler.timeout)
		handler.running = true

		go func() {
			for _ = range handler.timer.C {
				if handler.onTimeout != nil {
					handler.onTimeout()
				}
			}
		}()
	}
}

// Stop ...
func (handler *TimeoutHandlerModel) Stop() {
	if handler.running {
		handler.timer.Stop()
		handler.running = false
	}
}

// Running ...
func (handler TimeoutHandlerModel) Running() bool {
	return handler.running
}

// NewTimeoutHandler ...
func NewTimeoutHandler(timeout time.Duration, onTimeout func()) TimeoutHandlerModel {
	return TimeoutHandlerModel{
		timeout:   timeout,
		onTimeout: onTimeout,
	}
}

func runCommandInDiagnosticMode(command cmdex.CommandModel, checkPattern string, retryOnHang bool) error {
	log.Warn("Run in diagnostic mode")

	cmd := command.GetCmd()
	timeout := false

	// Create a timer that will FORCE kill the process if normal kill does not work
	var forceKillError error
	forceKillTimeoutHandler := NewTimeoutHandler(60*time.Second, func() {
		log.Warn("Timeout")
		timeout = true
		forceKillError = cmd.Process.Signal(syscall.SIGKILL)
	})
	// ----

	// Create a timer that will kill the process
	var killError error
	killTimeoutHandler := NewTimeoutHandler(300*time.Second, func() {
		log.Warn("Timeout")
		timeout = true
		forceKillTimeoutHandler.Start()
		killError = cmd.Process.Signal(syscall.SIGQUIT)
	})
	// ----

	// Redirect output
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdoutReader)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)

			killTimeoutHandler.Stop()
			if strings.Contains(strings.TrimSpace(line), checkPattern) {
				killTimeoutHandler.Start()
			}
		}
	}()
	if err := scanner.Err(); err != nil {
		return err
	}
	// ----

	if err := cmd.Start(); err != nil {
		return err
	}

	// Only proceed once the process has finished
	cmdErr := cmd.Wait()

	killTimeoutHandler.Stop()
	forceKillTimeoutHandler.Stop()

	if timeout {
		return fmt.Errorf("timed out")
	}
	if cmdErr != nil {
		return cmdErr
	}
	if killError != nil {
		return killError
	}
	if forceKillError != nil {
		return forceKillError
	}

	return nil
	// ----
}
