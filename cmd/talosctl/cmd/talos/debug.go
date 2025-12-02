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

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/reporter"
)

// TODO(laurazard):
// - set ENV variables
// - clean up

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
  talosctl -n 172.20.0.2 debug ./debug-tools.tar /bin/sh`,

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

			stream, err := c.DebugContainer(ctx,
				grpc.MaxCallRecvMsgSize(4*1024*1024), // 4 MiB
				grpc.MaxCallSendMsgSize(4*1024*1024),
			)
			if err != nil {
				return fmt.Errorf("failed to create debug container stream: %w", err)
			}

			rep := reporter.New()

			var (
				sendC    = make(chan *machine.DebugContainerRequest, 100)
				sendDone chan error
				recvDone = make(chan error, 1)

				stdinDone chan error

				exitCode int32 = -1
			)

			pullProgWriter := pullProgressWriter{
				reporter:     rep,
				ongoingPulls: pullJobs{},
			}

			stopCancelC := make(chan struct{})
			go func() {
				c := make(chan os.Signal, 1)
				signal.Notify(c, forwardedSignals...)
				select {
				case <-c:
					cancel()
				case <-stopCancelC:
				}
				signal.Stop(c)
			}()

			go func() {
				var pullComplete bool

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
					case *machine.DebugContainerResponse_PullProgress:
						if pullComplete {
							continue
						}

						pullProgress := msg.GetPullProgress()
						id := pullProgress.GetId()

						if id == "done" {
							pullComplete = true
							rep.Report(reporter.Update{
								Message: "Image pull complete",
								Status:  reporter.StatusSucceeded,
							})

							continue
						}

						current := pullProgress.GetCurrent()
						total := pullProgress.GetTotal()
						message := pullProgress.GetMessage()
						pullProgWriter.updateJob(id, message, current, total)

						pullProgWriter.printLayerProgress()

					case *machine.DebugContainerResponse_ContainerId:
						stopCancelC <- struct{}{}
						sigHandler(ctx, sendC)
						rep.Report(reporter.Update{
							Message: "Container ready",
							Status:  reporter.StatusSucceeded,
						})

						fmt.Println()

						if term.IsTerminal(int(os.Stdin.Fd())) {
							oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
							if err != nil {
								recvDone <- fmt.Errorf("failed to set terminal to raw mode: %w", err)

								return
							}
							defer func() {
								if oldState != nil {
									term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
								}
							}()
						}

						// container is ready, start reading stdin
						stdinDone = stdinReader(ctx, sendC)

						// start sendLoop
						if sendDone == nil {
							sendDone = sendLoop(ctx, stream, sendC)
						}

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

			rep.Report(reporter.Update{
				Message: "Sending container spec..",
				Status:  reporter.StatusRunning,
			})

			// send spec first
			spec := &machine.DebugContainerRequest{
				Request: &machine.DebugContainerRequest_Spec{
					Spec: &machine.DebugContainerSpec{
						Args: debugCmdFlags.args,
					},
				},
			}

			// check if arg is a tar archive
			imageArg := args[0]
			imageFile, err := os.Open(imageArg)
			if errors.Is(err, os.ErrNotExist) {
				spec.Request.(*machine.DebugContainerRequest_Spec).Spec.ImageRef = imageArg
			}
			defer imageFile.Close() //nolint:errcheck

			if err := stream.Send(spec); err != nil {
				return fmt.Errorf("failed to send spec: %w", err)
			}

			rep.Report(reporter.Update{
				Message: "Container spec sent",
				Status:  reporter.StatusSucceeded,
			})

			if imageFile != nil {
				if err := sendImage(ctx, stream, args[0], rep); err != nil {
					return fmt.Errorf("failed to send image: %w", err)
				}
			}

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

				// if we're done piping stdin, close send stream and wait
				// for the server to exit which will cause recvLoop to return
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

				cancel() // cancels sigHandler, stdinReader goroutines

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
		})
	},
}

func sendImage(ctx context.Context, stream machine.MachineService_DebugContainerClient, path string, rep *reporter.Reporter) error {
	imageFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open image file: %w", err)
	}
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
		rep.Report(reporter.Update{
			Message: fmt.Sprintf("Uploading image: %.1f%% (%s / %s)", progress, formatBytes(totalSent), formatBytes(fileInfo.Size())),
			Status:  reporter.StatusRunning,
		})
	}

	rep.Report(reporter.Update{
		Message: fmt.Sprintf("Image uploaded (%s)", formatBytes(fileInfo.Size())),
		Status:  reporter.StatusSucceeded,
	})

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

type pullJob struct {
	ID      string
	Message string
	Current int64
	Total   int64
}

func (p *pullJob) progress() float64 {
	return float64(p.Current) / float64(p.Total) * 100
}

type pullJobs []*pullJob

func (p pullJobs) Len() int {
	return len(p)
}

func (p pullJobs) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p pullJobs) Less(i, j int) bool {
	return p[i].ID < p[j].ID
}

type pullProgressWriter struct {
	reporter     *reporter.Reporter
	ongoingPulls pullJobs
}

func (p *pullProgressWriter) updateJob(id, message string, current, total int64) {
	for _, job := range p.ongoingPulls {
		if job.ID == id {
			job.Message = message
			job.Current = current
			job.Total = total

			return
		}
	}

	p.ongoingPulls = append(p.ongoingPulls, &pullJob{
		ID:      id,
		Message: message,
		Current: current,
		Total:   total,
	})
}

func (p *pullProgressWriter) printLayerProgress() {
	sort.Sort(p.ongoingPulls)

	sb := strings.Builder{}
	sb.WriteString("Pulling image:\n")

	for _, job := range p.ongoingPulls {
		if job.Message == "Downloading" {
			fmt.Fprintf(&sb, "%s: %s (%.1f%%)\n", job.ID, job.Message, job.progress())

			continue
		}

		if job.Message == "Extracting" {
			fmt.Fprintf(&sb, "%s: %s (%ds)\n", job.ID, job.Message, job.Current)

			continue
		}

		fmt.Fprintf(&sb, "%s: %s\n", job.ID, job.Message)
	}

	p.reporter.Report(reporter.Update{
		Message: sb.String(),
		Status:  reporter.StatusRunning,
	})
}
