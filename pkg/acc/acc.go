package acc

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

// TaskScope exposes various operations and symbols for defining
// a program.
type TaskScope interface {
	Do(name string, task TaskStepRun) TaskStep
	Defer(name string, task TaskStepRun)
	Get(key string) Ref
	Cmd(path string, args ...interface{}) Command
}

// Task is the unit of execution. It typically
// corresponds 1:1 to a single task in a workflow job or a CI job.
type Task struct {
	Steps   []TaskStep
	Cleanup []TaskStep
}

func newStreams(jobName string) Streams {
	return Streams{
		m: map[string]StreamRef{
			"stdout": StreamRef{Job: jobName, Stream: "stdout"},
			"stderr": StreamRef{Job: jobName, Stream: "stderr"},
		},
	}
}

// TaskStepRun is an object that eventually wraps a Go function that
// is called when the task step is being run.
type TaskStepRun interface {
}

// TaskStep is a single step in a Task that is executed one by one.
// Each TaskStep has Name, which is used as breakpoints or for skipping steps on runtime.
type TaskStep struct {
	Name    string
	Run     TaskStepRun
	Streams Streams
	Outputs Values
}

func (j TaskStep) Get(key string) Ref {
	ref, err := j.Outputs.Get(key)
	if err != nil {
		f := getFrame(1)
		msg := fmt.Sprintf("%s:%d: %v", filepath.Base(f.File), f.Line, err)
		panic(msg)
	}

	return *ref
}

type Command struct {
	Path string
	Args []interface{}
}

type Func struct {
	Name    string
	F       func(ctx TaskStepContext) error
	Outputs []string
}

type TaskStepContext interface {
	// Set sets an output of the current task step
	Set(key, val string)

	// Get returns an input value for the current task step by key
	Get(key string) string

	// Cmd initializes an OS command to be executed
	Cmd(path string, args ...string) *TaskStepCmd

	// Exec executes the OS command
	Exec(command Command) (ExecResult, error)
}

type TaskStepCmd struct {
	cmd Command
	ctx TaskStepContext
}

func (cmd *TaskStepCmd) Exec() (ExecResult, error) {
	return cmd.ctx.Exec(cmd.cmd)
}

var _ TaskStepContext = &stepContext{}

type stepContext struct {
	setOutput func(key, val string)
	get       func(key string) string
	executor  Target
}

func (c *stepContext) Cmd(path string, args ...string) *TaskStepCmd {
	var iArgs []interface{}

	for _, a := range args {
		iArgs = append(iArgs, a)
	}

	return &TaskStepCmd{
		cmd: Command{
			Path: path,
			Args: iArgs,
		},
		ctx: c,
	}
}

func (c *stepContext) Exec(command Command) (ExecResult, error) {
	var args []string

	for _, a := range command.Args {
		args = append(args, fmt.Sprintf("%s", a))
	}

	var err error

	var res ExecResult

	func() {
		defer func() {
			if e := recover(); e != nil {
				err = fmt.Errorf("unhandled error: %v", e)
			}
		}()

		res = c.executor.Execute(command, args)
	}()

	return res, err
}

func (c *stepContext) Set(key, val string) {
	c.setOutput(key, val)
}

func (c *stepContext) Get(key string) string {
	return c.get(key)
}

var _ Target = &Runtime{}

// Runtime is the default implementation of Target that has useful features like
// command simulation.
type Runtime struct {
	ExecutionStubs []ExecutionStub

	// binDir is the filesystem path to the temporary directory
	// that stores shims to the command simulator.
	binDir string

	// GoTestName must be set to the current the value returned by *testing.T.Name() before calling Runtime.Start().
	// The value is used to redirect shim executions to the command simulator, so that
	// the simulator is able to run your code even in go-test.
	GoTestName string

	AllowByDefault bool
	Allowed        map[string]bool
	CommandPrinter CommandPrinter

	Stdout, Stderr io.Writer
}

func (t Runtime) GetStdout() io.Writer {
	return t.Stdout
}

func (t Runtime) GetStderr() io.Writer {
	return t.Stderr
}

const (
	InvocationEnv = "ACCTEST_INVOCATION"
	LogPrefix     = "AcctestInvocation"
)

type WrappedExitErr struct {
	ExitErr     *exec.ExitError
	CombinedBuf *bytes.Buffer
}

func (e WrappedExitErr) Error() string {
	return fmt.Sprintf(
		"%v\n%s\n%s",
		e.ExitErr,
		fmt.Sprintf("%s exit code: %d\n", LogPrefix, e.ExitErr.ExitCode()),
		fmt.Sprintf("%s combined output: %s\n", LogPrefix, e.CombinedBuf.String()),
	)
}

type UnexpectedCommandError struct {
	Command        Command
	CommandPrinter CommandPrinter
	Args           []string
}

type CommandPrinter interface {
	Sprint(Command) string
}

type goCommandPrinter struct {
}

func (p goCommandPrinter) Sprint(cmd Command) string {
	var args []string

	for _, a := range cmd.Args {
		args = append(args, fmt.Sprintf("%q", a))
	}
	return fmt.Sprintf(`Command{
  Path: %q,
  Args: []interface{}{
    %s,
  },
}`, cmd.Path, strings.Join(args, ", \n    "))
}

func (e UnexpectedCommandError) Error() string {
	args := e.Args

	msg := fmt.Sprintf(
		"command %s is not expected for execution. Add the below as expectation:\n%s\n",
		fmt.Sprintf("%s %s", e.Command.Path, strings.Join(args, " ")),
		e.CommandPrinter.Sprint(e.Command),
	)

	return msg
}

func (t Runtime) Execute(cmd Command, args []string) ExecResult {
	var c *exec.Cmd

	var path string

	ex := t.findExpected(cmd.Path, args)
	if ex == nil {
		if t.AllowByDefault || (t.Allowed != nil && t.Allowed[filepath.Base(cmd.Path)]) {
			path = cmd.Path

			c = exec.Command(path, args...)

			c.Env = []string{"PATH=" + t.binDir}
		} else {
			commandPrinter := t.CommandPrinter
			if commandPrinter == nil {
				commandPrinter = goCommandPrinter{}
			}
			panic(
				UnexpectedCommandError{
					CommandPrinter: commandPrinter,
					Command:        cmd,
					Args:           args,
				},
			)
		}
	} else {
		path = filepath.Join(t.binDir, filepath.Base(ex.Command.Path))

		c = exec.Command(path, args...)

		c.Env = []string{"PATH=" + t.binDir, InvocationEnv + "=" + ex.Command.Path}
	}

	res, err := RunExecCmd(c)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, LogPrefix) {
			var matched string

			for _, line := range strings.Split(err.Error(), "\n") {
				if !strings.Contains(line, LogPrefix) {
					continue
				}

				tokens := strings.Split(line, ": ")
				lastToken := tokens[len(tokens)-1]
				if lastToken == "command not found" {
					matched = tokens[len(tokens)-2]
					break
				} else if lastToken == "executable file not found in $PATH" {
					matched = strings.TrimRight(tokens[len(tokens)-2], `"`)
					matched = strings.TrimLeft(matched, `"`)
				}
			}

			env := os.Environ()
			println(env)

			if matched == "" {
				panic(err)
			}

			er := fmt.Errorf("%q executed by %q is not found in $PATH - this might be an unexpected execution or typo. Either add Expectation for %q to the test executor, or fix the typo in %q", matched, path, matched, matched)

			panic(er)
		} else if strings.Contains(err.Error(), "executable file not found") {
			er := fmt.Errorf("executable file %q is not found in $PATH - this might be an unexpected execution or typo. Either add Expectation for %q to the test executor, or fix the typo in %q", path, path, path)

			panic(er)
		}

		panic(err)
	}

	return *res
}

func (t Runtime) findExpected(path string, args []string) *ExecutionStub {
	for _, ex := range t.ExecutionStubs {
		if path != ex.Command.Path {
			continue
		}

		var exArgs []string

		for _, a := range ex.Command.Args {
			exArgs = append(exArgs, fmt.Sprintf("%s", a))
		}

		if !reflect.DeepEqual(args, exArgs) {
			continue
		}

		return &ex
	}

	return nil
}

func (t Runtime) Start() {
	if cmdName := os.Getenv(InvocationEnv); cmdName != "" {
		path := cmdName

		var args []string

		if os.Args[1] == "-test.run" {
			args = os.Args[3:]
		} else {
			args = os.Args[1:]
		}

		if args[0] == "--" {
			args = args[1:]
		}

		ex := t.findExpected(path, args)
		if ex == nil {
			fmt.Fprintf(os.Stderr, "Path %s args %v is not expected: %+v\n", path, args, t.ExecutionStubs)
			os.Exit(1)
		}

		ctx := RunContext{Stdout: os.Stdout}

		ex.Run(ctx)

		os.Exit(0)
	}

	for i, ex := range t.ExecutionStubs {
		binName := filepath.Base(ex.Command.Path)

		if ex.Command.Path == "" {
			panic(fmt.Errorf("bug: empty command path in command at %d: %+v", i, ex.Command))
		}

		wrapperPath := filepath.Join(t.binDir, binName)

		bashPath, err := exec.LookPath("bash")
		if err != nil {
			panic(fmt.Errorf("bash not found: %v", err))
		}

		var optionalExtraArgs string

		if t.GoTestName != "" {
			optionalExtraArgs = fmt.Sprintf("-test.run ^%s$ ", t.GoTestName)
		}

		if err := ioutil.WriteFile(wrapperPath, []byte(fmt.Sprintf(`#!%s -e
%s=%s %s %s-- "$@"
`, bashPath, InvocationEnv, ex.Command.Path, os.Args[0], optionalExtraArgs)), 0755); err != nil {
			panic(err)
		}
	}
}

type ExecutionStub struct {
	Command Command
	Run     func(ctx RunContext)
}

type RunContext struct {
	Stdout io.Writer
}

func (c RunContext) Exit(code int) {
	os.Exit(code)
}

type mapInputs struct {
	m map[string]string
}

func (m *mapInputs) get(key string) (string, error) {
	v, ok := m.m[key]
	if !ok {
		return "", fmt.Errorf("no input provided for key %q", key)
	}

	return v, nil
}

var _ Target = &FakeRuntime{}

type FakeRuntime struct {
	Stdout, Stderr bytes.Buffer
}

func (e *FakeRuntime) GetStdout() io.Writer {
	return &e.Stdout
}

func (e *FakeRuntime) GetStderr() io.Writer {
	return &e.Stderr
}

func (e *FakeRuntime) Execute(cmd Command, args []string) ExecResult {
	c := exec.Command("bash", "-c", "echo", fmt.Sprintf("%s %v", cmd.Path, args))

	if err := c.Start(); err != nil {
		panic(err)
	}

	res := ExecResult{
		Stdout: ioutil.NopCloser(&bytes.Buffer{}),
		Stderr: ioutil.NopCloser(&bytes.Buffer{}),
		Err:    nil,
	}

	if err := c.Wait(); err != nil {
		panic(err)
	}

	return res
}

type ExecResult struct {
	Stdout io.Reader
	Stderr io.Reader
	Err    error
}

// Target is the system the program is interacting against
type Target interface {
	Execute(cmd Command, args []string) ExecResult
	GetStdout() io.Writer
	GetStderr() io.Writer
}

func getFrame(skipFrames int) runtime.Frame {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := skipFrames + 2

	// Set size to targetFrameIndex+2 to ensure we have room for one more caller than we need
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)

	frame := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()
			if frameIndex == targetFrameIndex {
				frame = frameCandidate
			}
		}
	}

	return frame
}
