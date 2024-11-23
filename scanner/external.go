package scanner

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/navidrome/navidrome/log"
	. "github.com/navidrome/navidrome/utils/gg"
)

// scannerExternal is a scanner that runs an external process to do the scanning. It is used to avoid
// memory leaks or retention in the main process, as the scanner can consume a lot of memory. The
// external process will be spawned with the same executable as the current process, and will run
// the "scan" command with the "--subprocess" flag.
//
// The external process will send progress updates to the main process through its STDOUT, and the main
// process will forward them to the caller.
type scannerExternal struct {
}

func (s *scannerExternal) scanAll(ctx context.Context, fullRescan bool, progress chan<- *ProgressInfo) {
	exe, err := os.Executable()
	if err != nil {
		progress <- &ProgressInfo{Err: fmt.Errorf("failed to get executable path: %w", err)}
		return
	}
	log.Debug(ctx, "Spawning external scanner process", "fullRescan", fullRescan, "path", exe)
	cmd := exec.CommandContext(ctx, exe, "scan", "--nobanner", "--noconfig", "--subprocess", If(fullRescan, "--full", ""))

	in, out := io.Pipe()
	defer in.Close()
	defer out.Close()
	cmd.Stdout = out
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		progress <- &ProgressInfo{Err: fmt.Errorf("failed to start scanner process: %w", err)}
		return
	}
	go s.wait(cmd, out)

	decoder := gob.NewDecoder(in)
	for {
		var p ProgressInfo
		if err := decoder.Decode(&p); err != nil {
			if !errors.Is(err, io.EOF) {
				progress <- &ProgressInfo{Err: fmt.Errorf("failed to read status from scanner: %w", err)}
			}
			break
		}
		progress <- &p
	}
}

func (s *scannerExternal) wait(cmd *exec.Cmd, out *io.PipeWriter) {
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			_ = out.CloseWithError(fmt.Errorf("%s exited with non-zero status code: %w", cmd, exitErr))
		} else {
			_ = out.CloseWithError(fmt.Errorf("waiting %s cmd: %w", cmd, err))
		}
		return
	}
	_ = out.Close()
}

var _ scanner = (*scannerExternal)(nil)