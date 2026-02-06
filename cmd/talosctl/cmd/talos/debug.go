// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/siderolabs/gen/channel"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

var debugCmdFlags struct {
	imageCmdFlagsType

	args []string
}

func init() {
	debugCmd.Flags().StringSliceVar(&debugCmdFlags.args, "args", nil, "arguments to pass to the container")
	debugCmd.Flags().StringVar(&debugCmdFlags.namespace, "namespace", "inmem", "namespace to use: `system` (CRI containerd) or `inmem` for in-memory containerd instance")

	addCommand(debugCmd)
}

// debugCmd represents the debug command.
var debugCmd = &cobra.Command{
	Use:   "debug <image-tar-path|image ref> [args]",
	Short: "Run a debug container from an image archive or reference",
	Example: `  # Run a debug container from a local tar archive (image will be loaded into Talos from the archive)
    talosctl debug ./debug-tools.tar --args /bin/sh

  # Run a debug container from an image reference (Talos will pull the image if not present)
    talosctl debug docker.io/library/alpine:latest --args /bin/sh`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClientAndNodes(func(ctx context.Context, c *client.Client, nodes []string) error {
			if len(nodes) != 1 {
				return fmt.Errorf("expected exactly one node, got %v", nodes)
			}

			ctx = client.WithNode(ctx, nodes[0])

			rep := reporter.New()

			ctrdInstance, err := debugCmdFlags.containerdInstance()
			if err != nil {
				return err
			}

			// verify if we are sending a tarball or pulling an image
			_, err = os.Stat(args[0])
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to stat image argument: %w", err)
			}

			var imgName string

			if err == nil {
				imgName, err = imageImportInternal(ctx, c, ctrdInstance, nodes[0], args[0], rep)
				if err != nil {
					return fmt.Errorf("failed to import image: %w", err)
				}
			} else {
				pullResult, err := imagePullInternal(ctx, c, ctrdInstance, nodes, args[0], rep)
				if err != nil {
					return fmt.Errorf("failed to pull image: %w", err)
				}

				imgName = pullResult[nodes[0]]
			}

			// no easy way to disable hooking up signal handling to the command context,
			// so instead save this context and use a new one from here on out.
			//
			// new context so that SIGINT/similar won't immediately cancel streaming
			// and instead allow us to forward the signal to the container
			ctx = context.WithoutCancel(ctx)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			runStream, err := c.DebugClient.ContainerRun(ctx,
				grpc.MaxCallRecvMsgSize(4*1024*1024), // 4 MiB
				grpc.MaxCallSendMsgSize(4*1024*1024),
			)
			if err != nil {
				return fmt.Errorf("failed to create debug container stream: %w", err)
			}

			return runContainer(ctx, rep, runStream, imgName, debugCmdFlags.args, ctrdInstance)
		})
	},
}

//nolint:gocyclo,cyclop
func runContainer(
	ctx context.Context,
	rep *reporter.Reporter,
	stream grpc.BidiStreamingClient[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse],
	imageName string,
	args []string,
	ctrdInstance *common.ContainerdInstance,
) error { //nolint:gocyclo,cyclop
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	err := stream.Send(&machine.DebugContainerRunRequest{
		Request: &machine.DebugContainerRunRequest_Spec{
			Spec: &machine.DebugContainerRunRequestSpec{
				Containerd: ctrdInstance,
				ImageName:  imageName,
				Args:       args,
				Profile:    machine.DebugContainerRunRequestSpec_PROFILE_PRIVILEGED,
				Tty:        isTTY,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send container spec: %w", err)
	}

	if isTTY {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to set terminal to raw mode: %w", err)
		}

		defer func() {
			if oldState != nil {
				term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
			}
		}()
	}

	var (
		sendC    = make(chan *machine.DebugContainerRunRequest, 100)
		sendDone chan error
		recvDone = make(chan error, 1)

		stdinDone chan error

		exitCode int32 = -1
	)

	sigHandler(ctx, sendC)

	stdinDone = stdinReader(ctx, sendC)
	sendDone = sendLoop(stream, sendC)

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if status.Code(err) == codes.Canceled {
					recvDone <- context.Canceled

					return
				}

				recvDone <- err

				return
			}

			switch msg.Resp.(type) {
			case *machine.DebugContainerRunResponse_StdoutData:
				if stdoutData := msg.GetStdoutData(); stdoutData != nil {
					os.Stdout.Write(stdoutData) //nolint:errcheck
				}

			case *machine.DebugContainerRunResponse_ExitCode:
				exitCode = msg.GetExitCode()

				recvDone <- io.EOF

				return

			default:
				fmt.Fprintf(os.Stderr, "unknown message type %T\n", msg.Resp)
			}
		}
	}()

	// either stdin closes first, and we cancel goroutines and CloseSend this end of the stream
	// or recvLoop exits first (container exit or error), and we cancel goroutines and wait
	// for stdinReader to finish
	select {
	case err := <-stdinDone:
		if err != nil && err != io.EOF {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}

		cancel() // cancels sigHandler, stdinReader goroutines

		close(sendC)

		if sendDone != nil {
			<-sendDone
		}

		// close send stream and wait for the server to exit
		// which will cause recvLoop to return
		if err := stream.CloseSend(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close send stream: %v\n", err)
		}

		if recvErr := <-recvDone; recvErr != nil && recvErr != io.EOF {
			return recvErr
		}

	case err := <-recvDone:
		if err != nil {
			if err == context.Canceled {
				rep.Report(reporter.Update{
					Message: "context canceled",
					Status:  reporter.StatusError,
				})

				return nil
			} else if err != io.EOF {
				rep.Report(reporter.Update{
					Message: fmt.Sprintf("error: %s", err.Error()),
					Status:  reporter.StatusError,
				})

				return nil
			}
		}

		cancel() // sigHandler, stdinReader goroutines

		if stdinDone != nil {
			<-stdinDone
		}

		close(sendC)

		if sendDone != nil {
			<-sendDone
		}
	}

	if exitCode != -1 && exitCode != 0 {
		return fmt.Errorf("container exited with code %d", exitCode)
	}

	return nil
}

var forwardedSignals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGTERM,
	syscall.SIGQUIT,
	syscall.SIGHUP,
	syscall.SIGWINCH,
}

// sigHandler registers signal handlers for the signals in `forwardedSignals`,
// and sends them to `msgC`.
//
// In case the signal received is `SIGWINCH`, the size of the user's terminal
// is queried and sent as a `DebugContainerTerminalResize` message, in order
// to resize the container's terminal.
func sigHandler(ctx context.Context, msgC chan<- *machine.DebugContainerRunRequest) {
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, forwardedSignals...)

	go func() {
		defer func() {
			defer signal.Stop(sigC)
		}()

		for {
			select {
			case <-ctx.Done():
				return

			case sig := <-sigC:
				if sig.(syscall.Signal) == syscall.SIGWINCH {
					if term.IsTerminal(int(os.Stdin.Fd())) {
						width, height, err := term.GetSize(int(os.Stdin.Fd()))
						if err != nil {
							continue
						}

						if !channel.SendWithContext(ctx, msgC,
							&machine.DebugContainerRunRequest{
								Request: &machine.DebugContainerRunRequest_TermResize{
									TermResize: &machine.DebugContainerTerminalResize{
										Width:  int32(width),
										Height: int32(height),
									},
								},
							}) {
							return
						}
					}

					continue
				}

				if !channel.SendWithContext(ctx, msgC, &machine.DebugContainerRunRequest{
					Request: &machine.DebugContainerRunRequest_Signal{
						Signal: int32(sig.(syscall.Signal)),
					},
				}) {
					return
				}
			}
		}
	}()

	sigC <- syscall.SIGWINCH // trigger an initial resize
}

// stdinReader reads from stdin and sends data to msgC.
//
// This implementation is unfortunately a bit complex. This is due to the fact
// that reading from stdin is a blocking syscall, which means if we just do
// `os.Stdin.Read` and send it's output to `msgC`, canceling `ctx` won't cause
// this goroutine to end (that will only happen when there's something to read
// from stdin, and the goroutine will loop around and check `ctx.Done()`.
// To address this, wrap `os.Stdin` in a `io.Pipe()` we can cancel, and launch a
// separate goroutine to close the pipe when `ctx` is canceled.
// This way, as soon as `ctx` is canceled, the pipe is closed and the main
// goroutine will get an `io.EOF` and exit.
func stdinReader(ctx context.Context, msgC chan<- *machine.DebugContainerRunRequest) chan error {
	r, w := io.Pipe()
	done := make(chan error)

	go func() {
		io.Copy(w, os.Stdin) //nolint:errcheck
		w.Close()            //nolint:errcheck
	}()

	go func() {
		<-ctx.Done()
		w.Close() //nolint:errcheck
	}()

	go func() {
		buf := make([]byte, 1024)

		for {
			n, err := r.Read(buf)
			if err != nil {
				done <- err

				return
			}

			if n == 0 {
				continue
			}

			if !channel.SendWithContext(ctx, msgC, &machine.DebugContainerRunRequest{
				Request: &machine.DebugContainerRunRequest_StdinData{
					StdinData: buf[:n],
				},
			}) {
				done <- ctx.Err()

				return
			}
		}
	}()

	return done
}

// sendLoop launches a goroutine that reads messages from msgC and sends them to
// the gRPC stream.
// The launched goroutine exits if ctx is canceled, or an error is returned while
// sending a message to the gRPC stream.
//
// sendLoop returns a channel that will receive the error (or nil) when the
// goroutine exits.
func sendLoop(
	stream grpc.BidiStreamingClient[machine.DebugContainerRunRequest, machine.DebugContainerRunResponse],
	msgC chan *machine.DebugContainerRunRequest,
) chan error {
	done := make(chan error)

	go func() {
		for msg := range msgC {
			err := stream.Send(msg)
			if err != nil {
				done <- err

				return
			}
		}

		done <- nil
	}()

	return done
}
