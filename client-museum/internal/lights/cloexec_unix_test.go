//go:build unix

package lights

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCloseOnExec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "port")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY, 0) // like go.bug.st/serial: without close-on-exec
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	flags := func() uintptr {
		f, _, _ := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
		return f
	}
	if flags()&syscall.FD_CLOEXEC != 0 {
		t.Fatal("the test file should start without close-on-exec")
	}
	closeOnExec(path)
	if flags()&syscall.FD_CLOEXEC == 0 {
		t.Error("the port should be close-on-exec, so programs the app starts don't inherit it")
	}
}
