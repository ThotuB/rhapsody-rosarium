//go:build unix

package lights

import (
	"os"
	"strconv"
	"syscall"
)

// closeOnExec keeps the open port out of programs the app starts. go.bug.st/serial opens ports
// without close-on-exec, so the windows' WebKit helpers inherited the port and held on to it after
// the board was unplugged or restarted, which made it come back under another name (ttyACM1).
func closeOnExec(path string) {
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	for _, f := range fds {
		if target, err := os.Readlink("/proc/self/fd/" + f.Name()); err == nil && target == path {
			if fd, err := strconv.Atoi(f.Name()); err == nil {
				syscall.CloseOnExec(fd)
			}
		}
	}
}
