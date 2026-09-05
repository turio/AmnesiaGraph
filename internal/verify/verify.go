package verify

import (
	"io"
	"os/exec"
	"runtime"
)

// Result is intentionally compact and is not persisted in graph state.
type Result struct {
	Total          int
	Passed         int
	FailedCommand  string
	FailedPosition int
	Err            error
}

// Success reports whether every verification command completed successfully.
func (r Result) Success() bool {
	return r.Err == nil
}

// Run executes commands from root in declared order and stops at the first
// non-zero result. Output is streamed to the supplied writers and never saved.
func Run(root string, commands []string, stdout, stderr io.Writer) Result {
	result := Result{Total: len(commands)}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	for index, commandText := range commands {
		var command *exec.Cmd
		if runtime.GOOS == "windows" {
			command = exec.Command("cmd.exe", "/C", commandText)
		} else {
			command = exec.Command("/bin/sh", "-c", commandText)
		}
		command.Dir = root
		command.Stdout = stdout
		command.Stderr = stderr
		if err := command.Run(); err != nil {
			result.FailedCommand = commandText
			result.FailedPosition = index + 1
			result.Err = err
			return result
		}
		result.Passed++
	}
	return result
}
