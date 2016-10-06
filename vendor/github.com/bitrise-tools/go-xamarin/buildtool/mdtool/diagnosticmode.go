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

func runCommandInDiagnosticMode(command cmdex.CommandModel, checkPattern string, retryOnHang bool) error {
	log.Warn("Run in diagnostic mode")

	cmd := command.GetCmd()
	timeout := false

	// Create a timer that will FORCE kill the process if normal kill does not work
	var forceKillError error
	var forceKillTimeoutHandler *time.Timer
	startForceKillTimeoutHandler := func() {
		forceKillTimeoutHandler = time.AfterFunc(60*time.Second, func() {
			log.Warn("Timeout")
			timeout = true
			forceKillError = cmd.Process.Signal(syscall.SIGKILL)
		})
	}
	// ----

	// Create a timer that will kill the process
	var killError error
	var killTimeoutHandler *time.Timer
	startKillTimeoutHandler := func() {
		killTimeoutHandler = time.AfterFunc(300*time.Second, func() {
			log.Warn("Timeout")
			timeout = true
			killError = cmd.Process.Signal(syscall.SIGQUIT)

			startForceKillTimeoutHandler()
		})
	}

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
				startKillTimeoutHandler()
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

	if cmdErr != nil {
		return cmdErr
	}

	if killError != nil {
		return killError
	}
	if forceKillError != nil {
		return forceKillError
	}

	if timeout {
		if retryOnHang {
			return runCommandInDiagnosticMode(command, checkPattern, false)
		}
		return fmt.Errorf("timed out")
	}

	return nil
	// ----
}
