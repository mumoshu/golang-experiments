package acc

import (
	"io/ioutil"
	"os/exec"
	"testing"
)

func TestRunExecCmd(t *testing.T) {
	cmd := exec.Command("bash", "-c", "echo foo")

	res, err := RunExecCmd(cmd)
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}

	soBytes, err := ioutil.ReadAll(res.Stdout)
	got := string(soBytes)
	want := "foo\n"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

