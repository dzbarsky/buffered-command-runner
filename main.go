package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

const quickCommandThreshold = 5 * time.Second

func main() {
	allowSilentFailure := true
	switch os.Args[1] {
	case "--allow-silent-failure":
	case "--no-allow-silent-failure":
		allowSilentFailure = false
	default:
		panic("First argument should be '--[no]-allow-silent-failure, was " + os.Args[1])
	}

	// Run the command under `script` so that it thinks it's connected to a real terminal.
	// This makes some commands (such as docker builds) way nicer.
	cmd := exec.Command("script", append([]string{"-q", "/dev/null"}, os.Args[2:]...)...)
	stdout, err := cmd.StdoutPipe()
	must(err)

	stderr, err := cmd.StderrPipe()
	must(err)

	// Atomic bool
	buffering := int32(1)

	// Disable buffering once we've waited long enough.
	go func() {
		<-time.After(quickCommandThreshold)
		atomic.StoreInt32(&buffering, 0)
	}()

	finalFlush := make(chan struct{})

	var wg sync.WaitGroup
	// Read from source into a buffer as long as we're buffering, then dump to dest.
	// finalFlush is another way to trigger the buffer to flush.
	bufferThenFlush := func(source io.Reader, dest *os.File) {
		wg.Add(1)
		sourceBuffering := true
		scanner := bufio.NewScanner(source)
		buf := &bytes.Buffer{}
		for scanner.Scan() {
			if sourceBuffering {
				sourceBuffering = atomic.LoadInt32(&buffering) == 1
				// Flush the existing buffer.
				if !sourceBuffering {
					buf.WriteTo(dest)
					buf = nil
				}
			}
			text := scanner.Text() + "\n"
			if sourceBuffering {
				buf.WriteString(text)
			} else {
				dest.WriteString(text)
			}
		}
		// If we exceeded our wait threshold, the buffer already got flushed.
		// So we only need the final flush here if we are flushing output
		// due to failure. This lets the caller decide whether to flush or not.
		<-finalFlush
		buf.WriteTo(dest)
		wg.Done()
	}
	go bufferThenFlush(stdout, os.Stdout)
	go bufferThenFlush(stderr, os.Stderr)

	err = cmd.Run()
	if err != nil {
		if !allowSilentFailure {
			// Command failed so let's trigger a final flush and wait for that to go through, then we can exit.
			close(finalFlush)
			wg.Wait()
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		os.Exit(-1)
	}
}
