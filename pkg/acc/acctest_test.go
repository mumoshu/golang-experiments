package acc

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

type TaskTestCase struct {
	Inputs map[string]string
	Stdout string
	Stderr string
}

func TestSimple(t *testing.T) {
	RunTaskTest(t,
		func(s TaskScope) {
			s.Do("say hello", s.Cmd("bash", "-c", "echo hello"))
		},
		TaskTestCase{
			Inputs: map[string]string{},
			Stdout: "hello\n",
		},
	)
	RunTaskTest(t,
		func(s TaskScope) {
			s.Do("say hello", s.Cmd("bash", "-c", "echo hello 1>&2"))
		},
		TaskTestCase{
			Inputs: map[string]string{},
			Stderr: "hello\n",
		},
	)
}

func RunTaskTest(t *testing.T, taskFunc func(TaskScope), testcases ...TaskTestCase) {
	t.Helper()

	for i := range testcases {
		tc := testcases[i]

		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			t.Helper()

			var (
				stdout, stderr bytes.Buffer
				builder        TaskBuilder
			)

			taskFunc(&builder)
			task := builder.Build()

			runtime := &Runtime{
				AllowByDefault: true,
				Stderr:         &stderr,
				Stdout:         &stdout,
			}

			RunTask(task, runtime, &mapInputs{m: tc.Inputs})

			if got := tc.Stdout; stdout.String() != got {
				t.Errorf("unexpected stdout: want %q, got %q", got, stdout.String())
			}

			if got := tc.Stderr; stderr.String() != got {
				t.Errorf("unexpected stderr: want %q, got %q", got, stderr.String())
			}
		})
	}
}

func Start(t *testing.T, e *Runtime) {
	e.GoTestName = t.Name()

	e.Start()
}

func TestGoRuntime(t *testing.T) {
	seed := "someseed"

	workflowYamlPath := fmt.Sprintf(".github/workflows/%s.yaml", seed)

	runtime := &Runtime{
		Allowed: map[string]bool{
			"bash": true,
		},
		ExecutionStubs: []ExecutionStub{
			{
				Command: Command{
					Path: "helm",
					Args: []interface{}{"upgrade", "--install", "../charts/actions-runner-controller", "someseed"},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("helm upgrade succeeded.\n"))
					ctx.Exit(0)
				},
			},
			{
				Command: Command{
					Path: "kubectl",
					Args: []interface{}{"apply", "-f", "testdata/"},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("kubectl-apply succeeded.\n"))
					ctx.Exit(0)
				},
			},
			{
				Command: Command{
					Path: "kubectl",
					Args: []interface{}{"wait", "-n", "actions-runner-system", "deploy/controller-manager"},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("kubectl-wait succeeded.\n"))
					ctx.Exit(0)
				},
			},
			{
				Command: Command{Path: "ghcp", Args: []interface{}{"empty-commit", "-u", "mumoshu", "-r", "actions-test", "-m", "empty commit 1", "-b", "main"}},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("kubectl-wait succeeded.\n"))
					ctx.Exit(0)
				},
			},
			{
				Command: Command{
					Path: "ghcp",
					Args: []interface{}{
						"commit",
						"-u",
						"mumoshu",
						"-r",
						"actions-test",
						"-m",
						"mpty commit 1",
						"-b",
						"main",
						workflowYamlPath,
					},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("kubectl-wait succeeded.\n"))
					ctx.Exit(0)
				},
			},
			{
				Command: Command{
					Path: "kind",
					Args: []interface{}{"create", "cluster", "--name", seed},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("kind-create-cluster succeeded.\n"))
					ctx.Exit(0)
				},
			},
			{
				Command: Command{
					Path: "kind",
					Args: []interface{}{"delete", "cluster", "--name", seed},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("kind-create-cluster succeeded.\n"))
					ctx.Exit(0)
				},
			},
		},
		binDir:     t.TempDir(),
		GoTestName: t.Name(),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}

	Start(t, runtime)

	taskBuilder := &TaskBuilder{}

	inputs := &mapInputs{
		m: map[string]string{
			"seed": seed,
		},
	}

	for key, _ := range inputs.m {
		taskBuilder.Inputs.Def(key, nil)
	}

	var err interface{}

	// When writing a script in a lang other than Go,
	// this line would be replaced with some parser invocation
	func() {
		defer func() {
			err = recover()
		}()

		MyScript(taskBuilder)
	}()

	if err != nil {
		t.Fatalf("analyzing script: %v", err)
	}

	task := taskBuilder.Build()

	fakeRuntime := &FakeRuntime{}

	RunTask(task, fakeRuntime, inputs)

	RunTask(task, runtime, inputs)

	var buf bytes.Buffer

	func() {
		defer func() {
			if e := recover(); e != nil {
				t.Fatalf("unhandled error: %+v", e)
			}
		}()

		WriteBashScript(
			task,
			&mapInputs{
				m: map[string]string{
					"seed": "${SEED}",
				},
			},
			&buf,
		)
	}()

	expectedLogs := fmt.Sprintf(`#!/usr/bin/env bash
set -e
if [ -z "${SEED}" ]; then echo "\${SEED} is empty.; exit 1; fi"
kind create cluster --name ${SEED}
helm upgrade --install ../charts/actions-runner-controller ${SEED}
kubectl apply -f testdata/
kubectl wait -n actions-runner-system deploy/controller-manager
ghcp empty-commit -u mumoshu -r actions-test -m empty commit 1 -b main
%s run-task-step "generate workflow"
ghcp commit -u mumoshu -r actions-test -m mpty commit 1 -b main ${GEN_YAMLPATH}
`, os.Args[0])

	got := buf.String()
	if got != expectedLogs {
		err := fmt.Errorf("WriteBashScript returned unexpected output. Update the expectation with the below if this is an expected change:\n`%s`\n", got)
		t.Error(err)
	}
}
