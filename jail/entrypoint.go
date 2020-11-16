package jail

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"go.sbk.wtf/runj/state"
)

const (
	execFifoFilename = "exec.fifo"
)

// SetupEntrypoint starts a runj-entrypoint process, which then will later be
// signalled through `runj start` to run the specified program in the jail.
// This indirection is necessary so that the STDIO for `runj create` is directed
// to that process.
// Note: this API is unstable; expect it to change.
func SetupEntrypoint(id string, argv []string) (*exec.Cmd, error) {
	path, err := createExecFifo(id)
	if err != nil {
		return nil, err
	}
	args := append([]string{id, path}, argv...)
	cmd := exec.Command("runj-entrypoint", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, cmd.Start()
}

// inspired by runc

// createExecFifo creates a fifo for communication between runj and
// runj-entrypoint.
// See runc/libcontainer/container_linux.go for a similar example
func createExecFifo(id string) (string, error) {
	path := fifoPath(id)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("fifo: exec fifo %s already exists", path)
	}
	// umask??
	if err := unix.Mkfifo(path, 0622); err != nil {
		return "", err
	}
	return path, nil
}

func fifoPath(id string) string {
	return filepath.Join(state.Dir(id), execFifoFilename)
}

func AwaitFifoOpen(ctx context.Context, id string) error {
	type openResult struct {
		file *os.File
		err  error
	}
	fifoOpened := make(chan openResult)
	go func() {
		f, err := fifoOpen(fifoPath(id))
		fifoOpened <- openResult{f, err}
		close(fifoOpened)
	}()
	select {
	case result := <-fifoOpened:
		if result.err != nil {
			return result.err
		}
		return handleFifoResult(result.file)
	case <-ctx.Done():
		return errors.New("fifo: timed out")
	}
}

func fifoOpen(path string) (*os.File, error) {
	flags := os.O_RDONLY
	f, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return nil, errors.Wrap(err, "fifo: open exec fifo for reading")
	}
	return f, nil
}

func handleFifoResult(f *os.File) error {
	defer f.Close()
	if err := readFromExecFifo(f); err != nil {
		return err
	}
	return os.Remove(f.Name())
}

func readFromExecFifo(execFifo io.Reader) error {
	data, err := ioutil.ReadAll(execFifo)
	if err != nil {
		return err
	}
	if len(data) <= 0 {
		return errors.New("cannot start an already running container")
	}
	return nil
}

// end
