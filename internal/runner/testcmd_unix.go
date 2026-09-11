//go:build !windows

package runner

// The commands a test runs, and what the shell makes of them, are the one
// thing about these tests that is not the same on every machine. They live
// here so that the tests themselves read the same wherever they run.
const (
	// cmdSleep stays running until something stops it.
	cmdSleep = "sleep 30"

	// cmdFail ends with a status of its own, which is cmdFailCode.
	cmdFail     = "exit 3"
	cmdFailCode = 3

	// cmdPrintDir prints the directory the command is running in, and
	// cmdPrintPort the PORT it was handed.
	cmdPrintDir  = "pwd"
	cmdPrintPort = "echo $PORT"

	// codeNotFound is what the shell reports for a command it cannot find.
	codeNotFound = 127

	// rootDir and absDir are absolute paths for the tests that only work out
	// a path and never go near the filesystem.
	rootDir = "/project"
	absDir  = "/srv/api"

	// cmdTrue does nothing and ends at once.
	cmdTrue = "true"

	// cmdTick keeps talking, so there is output to read while it runs.
	cmdTick = "while true; do echo tick; sleep 0.05; done"

	// envHome names the variable holding the home directory.
	envHome = "HOME"

	// testAddrExt is what a broadcaster's address is called here.
	testAddrExt = ".sock"
)
