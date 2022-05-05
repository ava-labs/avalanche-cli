package cmd

import (
	"strings"

	"github.com/shirou/gopsutil/process"
)

const (
	procName = "backend start"
)

func IsServerProcessRunning() (bool, error) {
	p2, err := process.Processes()
	if err != nil {
		return false, err
	}

	for _, p := range p2 {

		name, err := p.Cmdline()
		if err != nil {
			return false, err
		}
		if strings.Contains(name, procName) {
			return true, nil
		}
	}
	return false, nil
}
