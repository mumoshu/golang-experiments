package acc

import (
	"os"
	"strings"
	"testing"
)

func TestTestExecutor(t *testing.T) {
	tmpDir := t.TempDir()

	t.Logf("Running %s", strings.Join(os.Args, " "))
	t.Logf("Using tempdir %s", tmpDir)
	t.Logf("Test Name is %s", t.Name())

	testExecutor := &Runtime{
		ExecutionStubs: []ExecutionStub{
			{
				Command: Command{
					Path: "helm",
					Args: []interface{}{"upgrade", "--install", "stable/nginx", "nginx"},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("helm upgrade succeeded.\n"))
					ctx.Exit(0)
				},
			},
			{
				Command: Command{
					Path: "bash",
					Args: []interface{}{"-c", "helm upgrade --install stable/nginx nginx"},
				},
				Run: func(ctx RunContext) {
					ctx.Stdout.Write([]byte("bash helm upgrade succeeded.\n"))
					ctx.Exit(0)
				},
			},
		},
		binDir:     tmpDir,
		GoTestName: t.Name(),
	}

	testExecutor.Start()

	testExecutor.Execute(Command{
		Path: "helm",
		Args: []interface{}{"upgrade", "--install", "stable/nginx", Ref{
			Key: "name",
		}},
	}, []string{"upgrade", "--install", "stable/nginx", "nginx"})

	testExecutor.Execute(Command{
		Path: "bash",
		Args: []interface{}{"-c", "helm upgrade --install stable/nginx nginx"},
	}, []string{"-c", "helm upgrade --install stable/nginx nginx"})
}
