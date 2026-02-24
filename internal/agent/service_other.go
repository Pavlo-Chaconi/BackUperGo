//go:build !windows

package agent

import "fmt"

func RunService(opts Options) error {
	return fmt.Errorf("service mode is only supported on Windows")
}
