//go:build !unix

package lights

// closeOnExec is only needed on unix, see cloexec_unix.go: Windows handles aren't inherited by default.
func closeOnExec(path string) {}
