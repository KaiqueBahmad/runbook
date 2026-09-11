//go:build windows

package runner

// The commands a test runs, and what the shell makes of them, are the one
// thing about these tests that is not the same on every machine. They live
// here so that the tests themselves read the same wherever they run.
const (
	// cmdSleep stays running until something stops it.
	cmdSleep = "timeout /t 30 /nobreak >nul"

	// cmdFail ends with a status of its own, which is cmdFailCode.
	cmdFail     = "exit /b 3"
	cmdFailCode = 3

	// cmdPrintDir prints the directory the command is running in, and
	// cmdPrintPort the PORT it was handed.
	cmdPrintDir  = "cd"
	cmdPrintPort = "echo %PORT%"

	// codeNotFound is what cmd.exe reports for a command it cannot find. It
	// does not have the shell's 127.
	codeNotFound = 1

	// rootDir and absDir are absolute paths for the tests that only work out
	// a path and never go near the filesystem.
	rootDir = `C:\project`
	absDir  = `C:\srv\api`

	// cmdTrue does nothing and ends at once.
	cmdTrue = "exit 0"

	// cmdTick keeps talking, so there is output to read while it runs.
	cmdTick = `powershell -Command "while($true){echo tick; Start-Sleep -Milliseconds 50}"`

	// envHome names the variable holding the home directory.
	envHome = "USERPROFILE"

	// testAddrExt is what a broadcaster's address is called here.
	testAddrExt = ".port"
)
