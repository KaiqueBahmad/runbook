//go:build windows

package state

import (
	"fmt"
	"strconv"
	"syscall"
	"unsafe"
)

var (
	modkernel32           = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess       = modkernel32.NewProc("OpenProcess")
	procGetProcessTimes   = modkernel32.NewProc("GetProcessTimes")
	procGetExitCodeProc   = modkernel32.NewProc("GetExitCodeProcess")
	procCloseHandle       = modkernel32.NewProc("CloseHandle")
)

const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	PROCESS_QUERY_INFORMATION         = 0x0400
	STILL_ACTIVE                      = 259
)

// Alive reports whether the recorded process is still running, and is still the
// same one. On Windows, we check if the process exists and has not exited.
// The start time (Boot field) is compared against the process creation time
// to detect PID reuse.
func (st State) Alive() bool {
	h, _, _ := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_LIMITED_INFORMATION|PROCESS_QUERY_INFORMATION),
		0,
		uintptr(st.PID),
	)
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)

	// Check if the process has exited.
	var exitCode uint32
	r, _, _ := procGetExitCodeProc.Call(h, uintptr(unsafe.Pointer(&exitCode)))
	if r == 0 {
		return false
	}
	if exitCode != STILL_ACTIVE {
		return false
	}

	// If we have a boot time recorded, verify it matches. Windows doesn't
	// have clock ticks like Linux, so we use the creation time converted
	// to a string for comparison.
	if st.Boot != "" {
		creationTime, _, err := getProcessTimes(h)
		if err != nil {
			// Can't verify, trust the PID is alive.
			return true
		}
		boot := strconv.FormatInt(creationTime, 10)
		return st.Boot == boot
	}

	return true
}

// ProcessBoot returns a string identifying when the process was created.
// On Windows, this is the creation time as a Unix timestamp in nanoseconds.
func ProcessBoot(pid int) (string, error) {
	h, _, err := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_LIMITED_INFORMATION),
		0,
		uintptr(pid),
	)
	if h == 0 {
		return "", fmt.Errorf("opening process %d: %v", pid, err)
	}
	defer procCloseHandle.Call(h)

	creationTime, _, err := getProcessTimes(h)
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(creationTime, 10), nil
}

// getProcessTimes returns the creation and exit times of a process handle.
// The creation time is returned as a Unix timestamp in nanoseconds.
func getProcessTimes(h uintptr) (int64, int64, error) {
	var creation, exit, kernel, user int64
	r, _, err := procGetProcessTimes.Call(
		h,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r == 0 {
		return 0, 0, fmt.Errorf("GetProcessTimes: %v", err)
	}

	// Windows FILETIME is 100-nanosecond intervals since January 1, 1601.
	// Convert to Unix timestamp.
	const windowsEpochToUnix = 116444736000000000
	creationUnix := (creation - windowsEpochToUnix) / 10 // Convert to nanoseconds
	exitUnix := (exit - windowsEpochToUnix) / 10

	return creationUnix, exitUnix, nil
}
