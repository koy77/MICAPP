package main

// xclip helpers with a hard timeout.
//
// The capture pipeline copies to the clipboard before it simulates Ctrl+V, so
// a stuck xclip (no clipboard manager, X hiccup) silently freezes the whole
// pipeline. These helpers log + kill xclip instead of waiting forever.

import (
	"fmt"
	"log"
	"os/exec"
	"time"
)

// clipWrite pipes data into xclip for the given X selection with a timeout.
func clipWrite(selection, mimeType string, data []byte, timeout time.Duration) error {
	cmd := exec.Command("xclip", "-selection", selection, "-t", mimeType)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	writeErr := make(chan error, 1)
	go func() {
		_, werr := stdin.Write(data)
		cerr := stdin.Close()
		if werr != nil {
			writeErr <- werr
			return
		}
		writeErr <- cerr
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return err
		}
		select {
		case werr := <-writeErr:
			return werr
		default:
			return nil
		}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		log.Printf("xclip (%s/%s) timed out after %s — killed", selection, mimeType, timeout)
		crashf("xclip timeout: selection=%s mime=%s data=%dB timeout=%s (clipboard step stalled the pipeline)",
			selection, mimeType, len(data), timeout)
		return fmt.Errorf("xclip timed out after %s", timeout)
	}
}
