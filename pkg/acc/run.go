package acc

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// RunTask provides the inputs to the task and executes it against the target,
// so that some useful side-effects happen on the target.
func RunTask(p *Task, t Target, inputs *mapInputs) {
	state := map[string]map[string]string{}

	for _, instruction := range p.Steps {
		switch impl := instruction.Run.(type) {
		case Command:
			var args []string

			for _, a := range impl.Args {
				switch typed := a.(type) {
				case string:
					args = append(args, typed)
				case Ref:
					if typed.Job == "" {
						v, err := inputs.get(typed.Key)
						if err != nil {
							panic(fmt.Errorf("instruction %q: %v", instruction.Name, err))
						}
						args = append(args, v)
					} else {
						j, ok := state[typed.Job]
						if !ok {
							panic(fmt.Errorf("instruction %q: depends on %q but it is not yet executed", instruction.Name, typed.Job))
						}

						v, ok := j[typed.Key]
						if !ok {
							panic(fmt.Errorf("instruction %q does not have output named %q", typed.Job, typed.Key))
						}

						args = append(args, v)
					}
				default:
					panic(fmt.Errorf("unexpected type of arg: %T: %+v", a, a))
				}
			}

			res := t.Execute(impl, args)

			var (
				stdoutBuf, stderrBuf bytes.Buffer
			)

			stdout := io.TeeReader(res.Stdout, &stdoutBuf)
			stderr := io.TeeReader(res.Stderr, &stderrBuf)

			var wg sync.WaitGroup

			var err error

			var once sync.Once

			wg.Add(1)
			go func() {
				defer func() {
					if e := recover(); e != nil {
						once.Do(func() {
							err = fmt.Errorf("stdout: %w", e)
						})
					}
				}()
				defer wg.Done()

				if _, e := io.Copy(t.GetStdout(), stdout); e != nil {
					once.Do(func() {
						err = e
					})
				}
			}()

			wg.Add(1)
			go func() {
				defer func() {
					if e := recover(); e != nil {
						once.Do(func() {
							err = fmt.Errorf("stderr: %w", e)
						})
					}
				}()
				defer wg.Done()

				if _, e := io.Copy(t.GetStderr(), stderr); e != nil {
					once.Do(func() {
						err = e
					})
				}
			}()

			wg.Wait()

			if err != nil {
				panic(err)
			}

			state[instruction.Name] = map[string]string{
				"stdout": stdoutBuf.String(),
			}
		case Func:
			outputs := map[string]string{}

			var err error

			func() {
				defer func() {
					if e := recover(); e != nil {
						err = fmt.Errorf("unhandled error in func: %v", e)
					}
				}()

				err = impl.F(&stepContext{
					setOutput: func(key, val string) {
						outputs[key] = val
					},
					get: func(key string) string {
						v, err := inputs.get(key)
						if err != nil {
							panic(fmt.Errorf("instruction %q: %v", instruction.Name, err))
						}
						return v
					},
					executor: t,
				})
			}()

			if err != nil {
				panic(err)
			}

			state[instruction.Name] = outputs
		default:
			panic(fmt.Errorf("unsupported type of instruction: %T", impl))
		}
	}
}
