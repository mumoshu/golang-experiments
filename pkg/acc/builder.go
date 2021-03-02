package acc

import (
	"fmt"
	"path/filepath"
)

var _ TaskScope = &TaskBuilder{}

// TaskBuilder is the builder that exposes TaskScope for defining a task
// and is able to return the built task to be run.
type TaskBuilder struct {
	jobs        []TaskStep
	cleanupJobs []TaskStep

	Inputs Values
}

func (p *TaskBuilder) Cmd(path string, args ...interface{}) Command {
	return Command{
		Path: path,
		Args: args,
	}
}

func (p *TaskBuilder) Defer(name string, task TaskStepRun) {
	p.cleanupJobs = append(p.cleanupJobs, TaskStep{
		Name: name,
		Run:  task,
	})
}

func (p *TaskBuilder) Get(key string) Ref {
	ref, err := p.Inputs.Get(key)
	if err != nil {
		f := getFrame(1)
		msg := fmt.Sprintf("%s:%d: %v", filepath.Base(f.File), f.Line, err)
		panic(msg)
	}

	return *ref
}

func (p *TaskBuilder) Do(name string, task TaskStepRun) TaskStep {
	vals := Values{
		Job: name,
	}

	switch impl := task.(type) {
	case Command:
		vals.Def("stdout", nil)
		vals.Def("stderr", nil)
	case Func:
		for _, key := range impl.Outputs {
			vals.Def(key, nil)
		}
	default:
		panic(fmt.Errorf("unsupported type of task: %T: %v", impl, impl))
	}

	job := TaskStep{
		Name:    name,
		Run:     task,
		Streams: newStreams(name),
		Outputs: vals,
	}

	p.jobs = append(p.jobs, job)

	return job
}

func (p *TaskBuilder) Build() *Task {
	return &Task{
		Steps:   p.jobs,
		Cleanup: p.cleanupJobs,
	}
}
