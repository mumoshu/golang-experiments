package acc

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// WriteBashScript compiles the function and writes the result as an executable bash script.
func WriteBashScript(p *Task, inputs *mapInputs, writer io.Writer) {
	state := map[string]map[string]string{}

	printf := func(format string, args ...interface{}) {
		fmt.Fprintf(writer, format+"\n", args...)
	}

	printf("#!/usr/bin/env bash")
	printf("set -e")

	for _, in := range inputs.m {
		printf(`if [ -z "%s" ]; then echo "\%s is empty.; exit 1; fi"`, in, in)
	}

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
							panic(fmt.Errorf("instruction %q: instruction %q does not have output named %q", instruction.Name, typed.Job, typed.Key))
						}

						args = append(args, v)
					}
				default:
					panic(fmt.Errorf("unexpected type of arg: %T: %+v", a, a))
				}
			}

			printf("%s %s", impl.Path, strings.Join(args, " "))

			stdout := "example stdout of " + instruction.Name
			state[instruction.Name] = map[string]string{
				"stdout": stdout,
			}
		case Func:
			outputs := map[string]string{}
			//
			//impl.F(&stepContext{
			//	setOutput: func(key, val string) {
			//		outputs[key] = val
			//	},
			//	get: func(key string) string {
			//		v, err := inputs.get(key)
			//		if err != nil {
			//			panic(fmt.Errorf("instruction %q: %v", instruction.Name, err))
			//		}
			//		return v
			//	},
			//	executor:
			//})

			self := os.Args[0]

			for _, o := range impl.Outputs {
				outputs[o] = fmt.Sprintf("${%s_%s}", strings.ToUpper(impl.Name), strings.ToUpper(o))
			}

			printf("%s run-task-step %q", self, instruction.Name)

			state[instruction.Name] = outputs
		default:
			panic(fmt.Errorf("unsupported type of instruction: %T", impl))
		}
	}
}
