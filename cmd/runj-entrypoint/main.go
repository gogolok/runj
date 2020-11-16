/*
runj-entrypoint is a small helper program for starting OCI jails.  This program
is used for ensuring that the jail process's STDIO is hooked up to the same
STDIO streams used for `runj create`.  This program is started when `runj
create` is invoked, but blocks until `runj start` is invoked.

Unfortunately, this program works through indirection that is not obvious.  When
`runj create` is run, it creates a fifo (see mkfifo(2)) and then starts this
program, passing the jail ID, the path to the fifo, and the program that should
be invoked as arguments.  This program then opens the fifo for writing, which
should block to wait for the right time to actually exec into the target
program.  `runj start` will open the fifo for reading, which unblocks this
program and the jail process can start.

This program exec(2)s to jexec(8), which is then responsible for jail_attach(2)
and another exec(2) into the final target program.  The sequence of `exec(2)`
preserves the PID so that it can be the target of a future invocation of `runj
kill`.
*/
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

func main() {
	exit, err := _main()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exit)
}

var usage = errors.New("usage: runj-entrypoint JAIL-ID FIFO-PATH PROGRAM [ARGS...]")

func _main() (int, error) {
	if len(os.Args) < 4 {
		return 1, usage
	}
	jid := os.Args[1]
	fifoPath := os.Args[2]
	argv := os.Args[3:]

	// Block until `runj start` is invoked
	fifofd, err := unix.Open(fifoPath, unix.O_WRONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return 2, fmt.Errorf("failed to open fifo: %w", err)
	}
	if _, err := unix.Write(fifofd, []byte("0")); err != nil {
		return 3, fmt.Errorf("failed to write to fifo: %w", err)
	}

	// call unix.Exec (which is execve(2)) to replace this process with the jexec
	jexecPath, err := exec.LookPath("jexec")
	if err != nil {
		return 4, fmt.Errorf("failed to find jexec: %w", err)
	}
	if err := unix.Exec(jexecPath, append([]string{"jexec", jid}, argv...), unix.Environ()); err != nil {
		return 5, fmt.Errorf("failed to exec: %w", err)
	}
	return 0, nil
}