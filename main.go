package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"

	"github.com/creack/pty"
	"github.com/urfave/cli/v2"
)

//go:generate echo "Hello World"
func main() {
	cmdCtx, cancel := context.WithCancel(context.Background())

	cmdChan := make(chan os.Signal, 1)

	signal.Notify(cmdChan, os.Interrupt)

	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name: "cmd",
			},
		},
		Action: func(ctx *cli.Context) error {
			var wg sync.WaitGroup

			cmdSlice := ctx.StringSlice("cmd")

			wg.Add(len(cmdSlice))

			for _, cmd := range cmdSlice {
				go func(cmdStr string, c *context.Context) {
					defer wg.Done()

					execCmd := exec.CommandContext(*c, "sh", "-c", cmdStr)
					f, err := pty.Start(execCmd)
					if err != nil {
						panic(err)
					}
					io.Copy(os.Stdout, f)
					io.Copy(os.Stderr, f)
					//label := fmt.Sprintf("cmd %d", i)

					//execCmd := exec.CommandContext(*c, "sh", "-c", cmdStr)

					//stdoutWriter := NewPrefixWriter(os.Stdout, label)
					//stderrWriter := NewPrefixWriter(os.Stderr, label)

					//execCmd.Stdout = stdoutWriter
					//execCmd.Stderr = stderrWriter

					//err := execCmd.Run()
					//if err != nil {
					//	log.Fatal(err)
					//	return
					//}

				}(cmd, &cmdCtx)
			}

			wg.Wait()
			return nil
		},
	}

	go func() {
		<-cmdChan
		fmt.Println("Cleanup")
		cancel()
		os.Exit(1)
	}()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

type PrefixWriter struct {
	prefix string
	writer io.Writer
	mu     sync.Mutex
	buf    bytes.Buffer
}

func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		prefix: prefix,
		writer: w,
	}
}

func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	totalWritten := 0
	for len(p) > 0 {
		// Check for newline in p
		newlineIndex := bytes.IndexByte(p, '\n')
		if newlineIndex == -1 {
			// No newline found, write to buffer and return
			pw.buf.Write(p)
			totalWritten += len(p)
			break
		}

		// Write up to and including the newline
		line := p[:newlineIndex+1]
		p = p[newlineIndex+1:]

		// Write any buffered data
		if pw.buf.Len() > 0 {
			pw.buf.Write(line)
			_, err := fmt.Fprintf(pw.writer, "[%s] %s", pw.prefix, pw.buf.String())
			if err != nil {
				return totalWritten, err
			}
			pw.buf.Reset()
		} else {
			// No buffered data, write line directly
			_, err := fmt.Fprintf(pw.writer, "[%s] %s", pw.prefix, string(line))
			if err != nil {
				return totalWritten, err
			}
		}
		totalWritten += len(line)
	}
	return totalWritten, nil
}
