// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

var debugCmdFlags struct {
	args []string
}

func init() {
	debugCmd.Flags().StringSliceVar(&debugCmdFlags.args, "args", nil, "arguments to pass to the container")

	addCommand(debugCmd)
}

// debugCmd represents the debug command.
var debugCmd = &cobra.Command{
	Use:   "debug <image-tar-path|image ref> [args]",
	Short: "Run a debug container from an image archive or reference",
	Example: `  # Run a debug container from a local tar archive
    talosctl -n 172.20.0.2 debug ./debug-tools.tar --args /bin/sh

  # Run a debug container from an image reference
    talosctl -n 172.20.0.2 debug docker.io/library/alpine:latest --args /bin/sh`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithClient(func(ctx context.Context, c *client.Client) error {
			// no easy way to disable hooking up signal handling to the command context,
			// so instead save this context and use a new one from here on out.
			// cmdCtx := ctx
			// new context so that SIGINT/similar won't immediately cancel streaming
			// and instead allow us to forward the signal to the container

			ctx = client.WithNode(ctx, GlobalArgs.Nodes[0])
			ctx = context.WithoutCancel(ctx)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			stream, err := c.DebugContainerCreate(ctx,
				grpc.MaxCallRecvMsgSize(4*1024*1024), // 4 MiB
				grpc.MaxCallSendMsgSize(4*1024*1024),
			)
			if err != nil {
				return fmt.Errorf("failed to create debug container stream: %w", err)
			}

			rep := reporter.New()
			rep.Report(reporter.Update{
				Message: "Sending container spec..",
				Status:  reporter.StatusRunning,
			})

			ctrID, err := createContainer(ctx, rep, args[0], stream)
			if err != nil {
				return err
			}

			if err := stream.CloseSend(); err != nil {
				return err
			}

			runStream, err := c.DebugContainerRun(ctx,
				grpc.MaxCallRecvMsgSize(4*1024*1024), // 4 MiB
				grpc.MaxCallSendMsgSize(4*1024*1024),
			)
			if err != nil {
				return fmt.Errorf("failed to create debug container stream: %w", err)
			}

			if term.IsTerminal(int(os.Stdin.Fd())) {
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

			return runContainer(ctx, rep, runStream, ctrID)
		})
	},
}

func createContainer(ctx context.Context, rep *reporter.Reporter, imageArg string, stream machine.MachineService_DebugContainerCreateClient) (string, error) { //nolint:gocyclo
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	imageFile, err := os.Open(imageArg)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	} else {
		defer imageFile.Close() //nolint:errcheck
	}

	spec := &machine.DebugContainerCreateRequest_Spec{
		Spec: &machine.DebugContainerSpec{
			Args: debugCmdFlags.args,
		},
	}
	if imageFile == nil {
		spec.Spec.ImageRef = imageArg
	}

	if err := stream.Send(&machine.DebugContainerCreateRequest{
		Request: spec,
	}); err != nil {
		return "", fmt.Errorf("failed to send spec: %w", err)
	}

	rep.Report(reporter.Update{
		Message: "Container spec sent",
		Status:  reporter.StatusSucceeded,
	})

	if imageFile != nil {
		if err := sendImage(ctx, stream, imageFile, rep); err != nil {
			return "", fmt.Errorf("failed to send image: %w", err)
		}
	}

	pullProgWriter := pullProgressWriter{
		reporter:     rep,
		ongoingPulls: pullJobs{},
	}

	var pullComplete bool

	for {
		msg, err := stream.Recv()
		if err != nil {
			if status.Code(err) == codes.Canceled {
				return "", context.Canceled
			}

			return "", err
		}

		switch msg.Response.(type) {
		case *machine.DebugContainerCreateResponse_PullProgress:
			if pullComplete {
				continue
			}

			pullProgress := msg.GetPullProgress()
			id := pullProgress.GetLayerId()

			if id == "done" {
				pullComplete = true

				rep.Report(reporter.Update{
					Message: "Image pull complete",
					Status:  reporter.StatusSucceeded,
				})

				continue
			}

			pullProgWriter.updateJob(id, pullProgress.GetProgressStatus())

			pullProgWriter.printLayerProgress()

		case *machine.DebugContainerCreateResponse_ContainerId:
			rep.Report(reporter.Update{
				Message: fmt.Sprintf("Container created: %s\n", msg.GetContainerId()),
				Status:  reporter.StatusSucceeded,
			})

			return msg.GetContainerId(), nil

		default:
			fmt.Printf("Unknown type\n")
		}
	}
}

func sendImage(ctx context.Context, stream machine.MachineService_DebugContainerCreateClient, imageFile *os.File, rep *reporter.Reporter) error {
	defer imageFile.Close() //nolint:errcheck

	fileInfo, err := imageFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat image file: %w", err)
	}

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

		req := &machine.DebugContainerCreateRequest{
			Request: &machine.DebugContainerCreateRequest_ImageChunk{
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
		rep.Report(reporter.Update{
			Message: fmt.Sprintf("Uploading image: %.1f%% (%s / %s)", progress, formatBytes(totalSent), formatBytes(fileInfo.Size())),
			Status:  reporter.StatusRunning,
		})
	}

	rep.Report(reporter.Update{
		Message: fmt.Sprintf("Image uploaded (%s)", formatBytes(fileInfo.Size())),
		Status:  reporter.StatusSucceeded,
	})

	req := &machine.DebugContainerCreateRequest{
		Request: &machine.DebugContainerCreateRequest_ImageChunk{
			ImageChunk: &common.Data{
				Bytes: []byte{},
			},
		},
	}

	if err := stream.Send(req); err != nil {
		return fmt.Errorf("failed to send image chunk: %w", err)
	}

	return nil
}

func runContainer(ctx context.Context, rep *reporter.Reporter, stream machine.MachineService_DebugContainerRunClient, containerID string) error { //nolint:gocyclo,cyclop
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rep.Report(reporter.Update{
		Message: fmt.Sprintf("Starting container: %s\n", containerID),
		Status:  reporter.StatusRunning,
	})

	err := stream.Send(&machine.DebugContainerRunRequest{
		Request: &machine.DebugContainerRunRequest_ContainerId{
			ContainerId: containerID,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send container ID: %w", err)
	}
	// TODO(laurazard): stay attached until container removed

	rep.Report(reporter.Update{
		Message: fmt.Sprintf("Container started: %s\n", containerID),
		Status:  reporter.StatusSucceeded,
	})

	var (
		sendC    = make(chan *machine.DebugContainerRunRequest, 100)
		sendDone chan error
		recvDone = make(chan error, 1)

		stdinDone chan error

		exitCode int32 = -1
	)

	sigHandler(ctx, sendC)

	stdinDone = stdinReader(ctx, sendC)
	sendDone = sendLoop(ctx, stream, sendC)

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
				fmt.Printf("Unknown type\n")
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
					Message: fmt.Sprintf("Error: %s", err.Error()),
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
		rep.Report(reporter.Update{
			Message: fmt.Sprintf("Container exited with status code %d", exitCode),
			Status:  reporter.StatusSucceeded,
		})
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

// sigHandler registers signal handlers for the signals in
// `forwardedSignals`, and sends them to `msgC`.
//
// In case the signal received is `SIGWINCH`, the size of the user's
// terminal is queried and sent as a `DebugContainerTerminalResize`
// message, in order to resize the container's terminal.
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

						msgC <- &machine.DebugContainerRunRequest{
							Request: &machine.DebugContainerRunRequest_TermResize{
								TermResize: &machine.DebugContainerTerminalResize{
									Width:  int32(width),
									Height: int32(height),
								},
							},
						}
					}

					continue
				}

				msgC <- &machine.DebugContainerRunRequest{
					Request: &machine.DebugContainerRunRequest_Signal{
						Signal: int32(sig.(syscall.Signal)),
					},
				}
			}
		}
	}()

	sigC <- syscall.SIGWINCH // trigger an initial resize
}

// stdinReader reads from stdin and sends data to msgC.
//
// This implementation is unfortunately a bit complex. This is due
// to the fact that reading from stdin is a blocking syscall, which
// means if we just do `os.Stdin.Read` and send it's output to
// `msgC`, canceling `ctx` won't cause this goroutine to end (that
// will only happen when there's something to read from stdin, and
// the goroutine will loop around and check `ctx.Done()`.
// To address this, wrap `os.Stdin` in a `io.Pipe()` we can cancel,
// and launch a separate goroutine to close the pipe when `ctx` is
// canceled.
// This way, as soon as `ctx` is canceled, the pipe is closed and
// the main goroutine will get an `io.EOF` and exit.
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

			msgC <- &machine.DebugContainerRunRequest{
				Request: &machine.DebugContainerRunRequest_StdinData{
					StdinData: buf[:n],
				},
			}
		}
	}()

	return done
}

func sendLoop(ctx context.Context, stream machine.MachineService_DebugContainerRunClient, msgC chan *machine.DebugContainerRunRequest) chan error {
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

type pullJob struct {
	LayerID string
	Status  *machine.DebugContainerPullProgressStatus
}

type pullJobs []*pullJob

func (p pullJobs) Len() int {
	return len(p)
}

func (p pullJobs) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p pullJobs) Less(i, j int) bool {
	return p[i].LayerID < p[j].LayerID
}

type pullProgressWriter struct {
	reporter     *reporter.Reporter
	ongoingPulls pullJobs
}

func (p *pullProgressWriter) updateJob(layerID string, progress *machine.DebugContainerPullProgressStatus) {
	for _, job := range p.ongoingPulls {
		if job.LayerID == layerID {
			job.Status = progress

			return
		}
	}

	p.ongoingPulls = append(p.ongoingPulls, &pullJob{
		LayerID: layerID,
		Status:  progress,
	})
}

func (p *pullProgressWriter) printLayerProgress() {
	sort.Sort(p.ongoingPulls)

	sb := strings.Builder{}
	sb.WriteString("Pulling image:\n")

	for _, job := range p.ongoingPulls {
		fmt.Fprintf(&sb, "  %s", image.FmtStatus(job.Status))
	}

	p.reporter.Report(reporter.Update{
		Message: sb.String(),
		Status:  reporter.StatusRunning,
	})
}
