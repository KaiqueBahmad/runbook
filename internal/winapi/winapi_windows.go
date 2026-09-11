//go:build windows

package winapi

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess     = kernel32.NewProc("OpenProcess")
	procGetExitCodeProc = kernel32.NewProc("GetExitCodeProcess")
	procGetProcessTimes = kernel32.NewProc("GetProcessTimes")
	procCloseHandle     = kernel32.NewProc("CloseHandle")
)

const (
	queryLimited = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	stillActive  = 259    // what GetExitCodeProcess says of a running process
)

// Alive reports whether the process with the given pid is running. A process
// that has ended but whose handle is still open answers with the status it
// ended with rather than stillActive, so it is not mistaken for a live one.
func Alive(pid int) bool {
	h, err := open(pid)
	if err != nil {
		return false
	}
	defer procCloseHandle.Call(h)

	code, err := exitCode(h)
	return err == nil && code == stillActive
}

// CreationTime is when the kernel says a process started, as a number that
// only has to be compared with another of its own kind: Runbook records it
// alongside a pid so that a pid handed out again is not mistaken for the
// process it recorded.
func CreationTime(pid int) (string, error) {
	h, err := open(pid)
	if err != nil {
		return "", err
	}
	defer procCloseHandle.Call(h)

	var creation, exit, kernel, user int64
	r, _, err := procGetProcessTimes.Call(
		h,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r == 0 {
		return "", fmt.Errorf("the start time of %d: %v", pid, err)
	}
	// The number is in hundreds of nanoseconds since 1601, which nothing else
	// here reads, so it is handed on as it comes.
	return fmt.Sprint(creation), nil
}

// open takes a handle to a process, asking for no more than the right to be
// told about it.
func open(pid int) (uintptr, error) {
	h, _, err := procOpenProcess.Call(uintptr(queryLimited), 0, uintptr(pid))
	if h == 0 {
		return 0, fmt.Errorf("opening process %d: %v", pid, err)
	}
	return h, nil
}

// exitCode is the status a process ended with, or stillActive while it runs.
func exitCode(h uintptr) (uint32, error) {
	var code uint32
	if r, _, err := procGetExitCodeProc.Call(h, uintptr(unsafe.Pointer(&code))); r == 0 {
		return 0, fmt.Errorf("the exit code: %v", err)
	}
	return code, nil
}
