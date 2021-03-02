package acc

import "fmt"

type Values struct {
	Job   string
	exprs map[string]*Expr
}

type ValueNotDefinedError struct {
	TargetJob string
	Key       string
}

func (e *ValueNotDefinedError) Error() string {
	return fmt.Sprintf("job %q does not have %q defined", e.TargetJob, e.Key)
}

func (v *Values) Get(key string) (*Ref, error) {
	_, ok := v.exprs[key]
	if !ok {
		return nil, &ValueNotDefinedError{TargetJob: v.Job, Key: key}
	}

	return &Ref{Job: v.Job, Key: key}, nil
}

func (v *Values) Def(key string, expr *Expr) {
	if v.exprs == nil {
		v.exprs = map[string]*Expr{}
	}

	v.exprs[key] = expr
}

type Ref struct {
	Job string
	Key string
}

type Expr struct {
}

