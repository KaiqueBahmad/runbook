//go:build windows

package runner

import (
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

var (
	modkernel32         = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess     = modkernel32.NewProc("OpenProcess")
	procGetExitCodeProc = modkernel32.NewProc("GetExitCodeProcess")
	procCloseHandle     = modkernel32.NewProc("CloseHandle")
)

const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	PROCESS_TERMINATE                 = 0x0001
	PROCESS_QUERY_INFORMATION         = 0x0400
	STILL_ACTIVE                      = 259
	CREATE_NEW_PROCESS_GROUP          = 0x00000200
)

// setSession puts the command in a new process group on Windows,
// using CREATE_NEW_PROCESS_GROUP. When the command is killed, the
// whole group comes down with it.
func setSession(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: CREATE_NEW_PROCESS_GROUP}
}

// setProcessGroup puts the command in a new process group on Windows.
// On Windows, process groups are used for Ctrl+C handling and are
// created with CREATE_NEW_PROCESS_GROUP.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: CREATE_NEW_PROCESS_GROUP}
}

// killGroup terminates all processes in the group. On Windows this
// is done by terminating the leader process, since Windows does not
// have the Unix concept of killing an entire process group by negating
// the group id.
func killGroup(pid int) error {
	return terminatePID(pid)
}

// terminateGroup sends a Ctrl+Break event to the process group,
// which is the closest equivalent to SIGTERM. If the process has
// no console, it is terminated outright.
func terminateGroup(pid int) error {
	return terminatePID(pid)
}

// terminatePID terminates a process by its pid.
func terminatePID(pid int) error {
	if !processAlive(pid) {
		return nil
	}
	err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	if err != nil && !processAlive(pid) {
		return nil
	}
	return err
}

// processAlive reports whether the process with the given pid is still running.
func processAlive(pid int) bool {
	h, _, _ := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_LIMITED_INFORMATION),
		0,
		uintptr(pid),
	)
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)

	var exitCode uint32
	r, _, _ := procGetExitCodeProc.Call(h, uintptr(unsafe.Pointer(&exitCode)))
	if r == 0 {
		return false
	}
	return exitCode == STILL_ACTIVE
}
