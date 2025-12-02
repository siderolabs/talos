// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// TODO(laurazard):
// - image from stdin
// - image from ref (node pulls it)
// - set ENV variables
// - basic (T)UI
// - clean up
//
// DONE:
// - monitor signals + forward (SIGINT, SIGTERM, SIGQUIT, SIGHUP)
// - monitor terminal resize (SIGWINCH) + forward

var debugCmdFlags struct {
	args []string
}

// debugCmd represents the debug command.
var debugCmd = &cobra.Command{
	Use:   "debug <image-tar-path>",
	Short: "Upload and run a debug container from an image tar",
	Example: `  # Run a debug container from a local tar archive
  talosctl -n 172.20.0.2 debug ./debug-tools.tar

  # Run with custom arguments
  talosctl -n 172.20.0.2 debug ./debug-tools.tar --args "/bin/sh,-c,ls -la"`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			// no easy way to disable hooking up signal handling to the command context,
			// so instead save this context and use a new one from here on out.
			// cmdCtx := ctx
			// new context so that SIGINT/similar won't immediately cancel streaming
			// and instead allow us to forward the signal to the container
			ctx = client.WithNode(context.Background(), GlobalArgs.Nodes[0])
			ctx, cancel := context.WithCancel(ctx)

			stream, err := c.DebugContainer(ctx,
				grpc.MaxCallRecvMsgSize(4<<20), // 4 MiB
				grpc.MaxCallSendMsgSize(4<<20),
			)
			if err != nil {
				return fmt.Errorf("failed to create debug container stream: %w", err)
			}

			// send spec first
			spec := &machine.DebugContainerRequest{
				Request: &machine.DebugContainerRequest_Spec{
					Spec: &machine.DebugContainerSpec{
						Args: debugCmdFlags.args,
					},
				},
			}

			if err := stream.Send(spec); err != nil {
				return fmt.Errorf("failed to send spec: %w", err)
			}

			if err := sendImage(ctx, stream, args[0]); err != nil {
				return fmt.Errorf("failed to send image: %w", err)
			}

			fmt.Println("\nImporting/unpacking image...") // New line after progress

			var oldState *term.State
			if term.IsTerminal(int(os.Stdin.Fd())) {
				oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
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
				sendC    = make(chan *machine.DebugContainerRequest, 100)
				sendDone chan error
				recvDone = make(chan error, 1)

				stdinDone = stdinReader(ctx, sendC)

				exitCode int32 = -1
			)

			go func() {
				for {
					msg, err := stream.Recv()
					if err != nil {
						if status.Code(err) == codes.Canceled {
							recvDone <- nil

							return
						}

						recvDone <- err

						return
					}

					if sendDone == nil {
						// start sendLoop if it hasn't yet started
						sendDone = sendLoop(ctx, stream, sendC)
					}

					switch msg.Resp.(type) {
					case *machine.DebugContainerResponse_StdoutData:
						if stdoutData := msg.GetStdoutData(); stdoutData != nil {
							os.Stdout.Write(stdoutData) //nolint:errcheck
						}

					case *machine.DebugContainerResponse_ExitCode:
						exitCode = msg.GetExitCode()
						recvDone <- io.EOF

						return

					default:
						fmt.Printf("Unknown type\n")
					}
				}
			}()

			sigHandler(ctx, sendC)

			select {
			case err := <-stdinDone:
				if err != nil && err != io.EOF {
					fmt.Fprintf(os.Stderr, "%s\n", err.Error())
				}

				cancel() // cancels sigHandler, stdinReader goroutines

				close(sendC)
				<-sendDone

				// if we're done piping stdin, close send stream and wait
				// for the server to exit which will cause recvLoop to return
				if err := stream.CloseSend(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close send stream: %v\n", err)
				}

				if recvErr := <-recvDone; recvErr != nil && recvErr != io.EOF {
					return recvErr
				}

			case err := <-recvDone:
				if err != nil && err != io.EOF {
					return err
				}

				cancel() // cancels sigHandler, stdinReader goroutines
				<-stdinDone

				close(sendC)
				<-sendDone
			}

			if exitCode != -1 && exitCode != 0 {
				fmt.Fprintf(os.Stderr, "\nContainer exited with exit code: %d\n", exitCode)
			}

			return nil
		})
	},
}

func sendImage(ctx context.Context, stream machine.MachineService_DebugContainerClient, path string) error {
	imageFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open image file: %w", err)
	}
	defer imageFile.Close() //nolint:errcheck

	fileInfo, err := imageFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat image file: %w", err)
	}

	fmt.Printf("Uploading image from %s (%s)...\n", path, formatBytes(fileInfo.Size()))

	var (
		buf       = make([]byte, 1024*1024) // 1MB buffer
		totalSent int64
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := imageFile.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}

			return fmt.Errorf("failed to read image: %w", err)
		}

		if n == 0 {
			break
		}

		req := &machine.DebugContainerRequest{
			Request: &machine.DebugContainerRequest_ImageChunk{
				ImageChunk: &common.Data{
					Bytes: buf[:n],
				},
			},
		}

		if err := stream.Send(req); err != nil {
			return fmt.Errorf("failed to send image chunk: %w", err)
		}

		totalSent += int64(n)

		progress := float64(totalSent) / float64(fileInfo.Size()) * 100
		fmt.Printf("\rProgress: %.1f%% (%s / %s)", progress, formatBytes(totalSent), formatBytes(fileInfo.Size()))
	}

	return nil
}

var forwardedSignals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGTERM,
	syscall.SIGQUIT,
	syscall.SIGHUP,
}

func sigHandler(ctx context.Context, msgC chan<- *machine.DebugContainerRequest) {
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, forwardedSignals...)

	winchC := make(chan os.Signal, 1)
	signal.Notify(winchC, syscall.SIGWINCH)

	go func() {
		defer func() {
			defer signal.Stop(winchC)
			defer signal.Stop(sigC)
		}()

		for {
			select {
			case <-ctx.Done():
				return

			case <-winchC:
				if term.IsTerminal(int(os.Stdin.Fd())) {
					width, height, err := term.GetSize(int(os.Stdin.Fd()))
					if err != nil {
						continue
					}

					msgC <- &machine.DebugContainerRequest{
						Request: &machine.DebugContainerRequest_TermResize{
							TermResize: &machine.DebugContainerTerminalResize{
								Width:  int32(width),
								Height: int32(height),
							},
						},
					}
				}

			case sig := <-sigC:
				msgC <- &machine.DebugContainerRequest{
					Request: &machine.DebugContainerRequest_Signal{
						Signal: int32(sig.(syscall.Signal)),
					},
				}
			}
		}
	}()

	winchC <- syscall.SIGWINCH // just trigger an initial resize
}

func stdinReader(ctx context.Context, msgC chan<- *machine.DebugContainerRequest) chan error {
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

			msgC <- &machine.DebugContainerRequest{
				Request: &machine.DebugContainerRequest_StdinData{
					StdinData: buf[:n],
				},
			}
		}
	}()

	return done
}

func sendLoop(ctx context.Context, stream machine.MachineService_DebugContainerClient, msgC chan *machine.DebugContainerRequest) chan error {
	done := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return

			case msg := <-msgC:
				err := stream.Send(msg)
				if err != nil {
					done <- err

					return
				}
			}
		}
	}()

	return done
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	debugCmd.Flags().StringSliceVar(&debugCmdFlags.args, "args", nil, "arguments to pass to the container")

	addCommand(debugCmd)
}
