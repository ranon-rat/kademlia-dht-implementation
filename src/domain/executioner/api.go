package executionerD

import (
	"context"
	"os/exec"
)

type RunningProcess struct {
	Cmd    *exec.Cmd
	Cancel context.CancelFunc
}

type StartProcess struct {
	ID      uint64
	Command string
	Dir     string
}
