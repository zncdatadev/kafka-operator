package util

import "fmt"

// Subdirectory of the log directory containing files to control the Vector instance
const (
	VectorLogDir = "_vector"

	// File to signal that Vector should be gracefully shut down
	ShutdownFile = "shutdown"
)

const CommonBashTrapFunctions = `prepare_signal_handlers()
{
    unset term_child_pid
    unset term_kill_needed
    trap 'handle_term_signal' TERM
}

handle_term_signal()
{
    if [ "${term_child_pid}" ]; then
        kill -TERM "${term_child_pid}" 2>/dev/null
    else
        term_kill_needed="yes"
    fi
}

wait_for_termination()
{
    set +e
    term_child_pid=$1
    if [[ -v term_kill_needed ]]; then
        kill -TERM "${term_child_pid}" 2>/dev/null
    fi
    wait ${term_child_pid} 2>/dev/null
    trap - TERM
    wait ${term_child_pid} 2>/dev/null
    set -e
}
	`

// / Use this command to remove the shutdown file (if it exists) created by [`create_vector_shutdown_file_command`].
// / You should execute this command before starting your application.
func RemoveVectorShutdownFileCommand(logDir string) string {
	return fmt.Sprintf("rm -f %s/%s/%s", logDir, VectorLogDir, ShutdownFile)
}

// Command to create a shutdown file for the vector container.
// Please delete it before starting your application using `RemoveVectorShutdownFileCommand` .
func CreateVectorShutdownFileCommand(logDir string) string {
	return fmt.Sprintf("mkdir -p %s/%s && touch %s/%s/%s", logDir, VectorLogDir, logDir, VectorLogDir, ShutdownFile)
}
