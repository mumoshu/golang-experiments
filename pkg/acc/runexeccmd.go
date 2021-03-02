package acc

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"sync"
)

func RunExecCmd(cmd *exec.Cmd) (*ExecResult, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	combinedBuf := &bytes.Buffer{}

	stdout2 := io.TeeReader(stdout, combinedBuf)
	stderr2 := io.TeeReader(stderr, combinedBuf)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wg.Done()

		io.Copy(stdoutBuf, stdout2)
	}()

	wg.Add(1)
	go func() {
		wg.Done()

		io.Copy(stderrBuf, stderr2)
	}()

	if err := cmd.Wait(); err != nil {
		wg.Wait()

		exitErr := &exec.ExitError{}

		if errors.As(err, &exitErr) {
			return nil, WrappedExitErr{ExitErr: exitErr, CombinedBuf: combinedBuf}
		}

		return nil, err
	}

	wg.Wait()

	res := ExecResult{
		Stdout: stdoutBuf,
		Stderr: stderrBuf,
		Err:    nil,
	}

	return &res, nil
}

