//go:build windows

package gui

// The commands a test starts, and the names it looks things up by, are the one
// thing about these tests that is not the same on every machine. They live
// here so that the tests themselves read the same wherever they run.
const (
	// cmdSleep stays running until something stops it.
	cmdSleep = "timeout /t 30 /nobreak >nul"

	// cmdTick keeps talking, so there is output to read while it runs.
	cmdTick = `powershell -Command "while($true){echo tick; Start-Sleep -Milliseconds 50}"`

	// envHome names the variable holding the home directory.
	envHome = "USERPROFILE"

	// pathOutsideHome is an absolute path that no home directory contains.
	pathOutsideHome = `C:\etc\runbook.yml`
)
