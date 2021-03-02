package acc

import (
	"fmt"
	"io/ioutil"
)

// MyScript is a script written Go to produce a program
func MyScript(s TaskScope) {
	s.Defer("stop cluster",
		s.Cmd("kind", "delete", "cluster", "--name", s.Get("seed")),
	)

	s.Do("start cluster",
		s.Cmd("kind", "create", "cluster", "--name", s.Get("seed")),
	)

	s.Do("deploy controller",
		s.Cmd("helm", "upgrade", "--install", "../charts/actions-runner-controller", s.Get("seed")),
	)

	s.Do("deploy runners",
		s.Cmd("kubectl", "apply", "-f", "testdata/"),
	)

	s.Do("wait for runners",
		s.Cmd("kubectl", "wait", "-n", "actions-runner-system", "deploy/controller-manager"),
	)

	s.Do("trigger workflow run",
		s.Cmd("ghcp", "empty-commit", "-u", "mumoshu", "-r", "actions-test", "-m", "empty commit 1", "-b", "main"),
	)

	const (
		outputKeyYamlPath = "yamlPath"
	)

	genWorkflow := s.Do("generate workflow", Func{Name: "gen", Outputs: []string{outputKeyYamlPath}, F: func(ctx TaskStepContext) error {
		workflowYamlPath := fmt.Sprintf(".github/workflows/%s.yaml", ctx.Get("seed"))

		res, err := ctx.Cmd("bash", "-c", "echo test").Exec()
		if err != nil {
			return err
		}

		bs, err := ioutil.ReadAll(res.Stdout)
		if err != nil {
			return err
		}
		echoOut := string(bs)

		ctx.Set(outputKeyYamlPath, workflowYamlPath)
		ctx.Set("echoTest", echoOut)

		return nil
	}})

	s.Do(
		"setup workflow",
		s.Cmd(
			"ghcp",
			"commit",
			"-u", "mumoshu",
			"-r", "actions-test",
			"-m", "mpty commit 1",
			"-b", "main",
			genWorkflow.Get(outputKeyYamlPath),
		),
	)

	//s.WriteGit("mumoshu", "actions-test", "main", "testdata", testData)
}
