package admin

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/client/stillbox-go"
	"github.com/mattn/go-isatty"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v3"
	"github.com/urfave/sflags/gen/gcli"
)

func CallsCommand(cfg *config.Configuration) *cli.Command {
	c := &cfg.Config
	callsCmd := &cli.Command{
		Name:  "calls",
		Usage: "administers calls",
		Commands: []*cli.Command{
			moveCommand(c),
			gcCommand(c),
			fsckCommand(c),
		},
	}

	return callsCmd
}

type moveProgresser struct {
	total int64
	pb    *progressbar.ProgressBar
}

func (p *moveProgresser) textCb(msg callstore.MoveProgressMsg) {
	switch {
	case msg.Completed != nil:
		fmt.Printf("%d calls (%d%%) done...\n", *msg.Completed, int((float32(*msg.Completed)/float32(p.total))*100))
	case msg.Total != nil:
		p.total = *msg.Total
	case msg.Final != nil:
		fmt.Printf("Finished %d calls\n", *msg.Final)
	}
}

func (p *moveProgresser) ttyCb(msg callstore.MoveProgressMsg) {
	switch {
	case msg.Completed != nil:
		if p.pb == nil {
			break
		}
		_ = p.pb.Set64(*msg.Completed)
	case msg.Total != nil:
		if *msg.Total == 0 {
			break
		}
		p.pb = progressbar.NewOptions64(
			*msg.Total,
			progressbar.OptionSetDescription("moving calls"),
			progressbar.OptionSetItsString("calls"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetWidth(10),
			progressbar.OptionShowTotalBytes(true),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionOnCompletion(func() {
				_, _ = os.Stderr.WriteString("\n")
			}),
			progressbar.OptionSpinnerType(rand.Intn(75)),
			progressbar.OptionFullWidth(),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionEnableColorCodes(true),
		)
	case msg.Final != nil:
		if p.pb != nil {
			_ = p.pb.Set64(*msg.Final)
			_ = p.pb.Exit()
			p.pb = nil
		}
		fmt.Printf("Moved %d calls\n", *msg.Final)
	}
}

func moveCommand(cfg *config.Config) *cli.Command {
	params := &callstore.MoveCallParams{}

	c := &cli.Command{
		Name:        "move",
		Aliases:     []string{"mv"},
		Usage:       "moves calls between storage backends",
		Description: "moves calls between backends, including DB blob",
		UsageText:   "stillbox admin calls move",
		Flags:       []cli.Flag{},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cfg.Server.AdminSocket == nil {
				return fmt.Errorf("no admin socket configured")
			}

			prog := moveProgresser{}
			var progressCb func(msg callstore.MoveProgressMsg)
			if isatty.IsTerminal(os.Stdout.Fd()) {
				progressCb = prog.ttyCb
			} else {
				progressCb = prog.textCb
			}

			c, err := stillbox.NewRESTClient(stillbox.UnixSocket(*cfg.Server.AdminSocket))
			if err != nil {
				return err
			}

			// this is a hack needed because sflags doesn't set nils
			common.ZeroFields(params)

			err = c.MoveCalls(ctx, params, progressCb)
			if err != nil {
				if prog.pb != nil {
					_ = prog.pb.Exit()
				}
				return err
			}

			return nil
		},
	}

	flags, err := gcli.ParseV3(params)
	if err != nil {
		panic(err)
	}

	c.Flags = flags

	return c
}

func gcCommand(cfg *config.Config) *cli.Command {
	c := &cli.Command{
		Name:        "gc",
		Usage:       "garbage collects calls in journal",
		Description: "garbage collects calls in journal",
		UsageText:   "stillbox admin calls gc",
		Flags:       []cli.Flag{},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cfg.Server.AdminSocket == nil {
				return fmt.Errorf("no admin socket configured")
			}

			c, err := stillbox.NewRESTClient(stillbox.UnixSocket(*cfg.Server.AdminSocket))
			if err != nil {
				return err
			}

			return c.CallsGC(ctx)
		},
	}

	return c
}

func fsckCommand(cfg *config.Config) *cli.Command {
	params := new(callstore.FsckParams)
	c := &cli.Command{
		Name:        "fsck",
		Usage:       "check that all call references exist",
		Description: "check that all call references exist",
		UsageText:   "stillbox admin calls fsck",
		Flags:       []cli.Flag{},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cfg.Server.AdminSocket == nil {
				return fmt.Errorf("no admin socket configured")
			}

			c, err := stillbox.NewRESTClient(stillbox.UnixSocket(*cfg.Server.AdminSocket))
			if err != nil {
				return err
			}

			isTty := isatty.IsTerminal(os.Stdout.Fd())
			// this is a hack needed because sflags doesn't set nils
			common.ZeroFields(params)

			var pb *progressbar.ProgressBar
			progressCb := func(msg callstore.FsckReport) {
				fmt.Printf("%+v\n", msg)
			}

			if isTty {
				pb = progressbar.NewOptions64(
					-1, // indeterminate
					progressbar.OptionSetDescription("fscking calls"),
					progressbar.OptionSetWriter(os.Stderr),
					progressbar.OptionSetWidth(10),
					progressbar.OptionThrottle(65*time.Millisecond),
					progressbar.OptionSpinnerType(rand.Intn(75)),
					progressbar.OptionFullWidth(),
					progressbar.OptionOnCompletion(func() {
						_, _ = os.Stderr.WriteString("\n")
					}),
					progressbar.OptionSetRenderBlankState(true),
					progressbar.OptionShowCount(),
					progressbar.OptionEnableColorCodes(true),
				)
				progressCb = func(msg callstore.FsckReport) {
					if msg.Status != nil {
						pb.Describe(*msg.Status)
					}

					if msg.FinalCallsDangling != nil {
						pb.ChangeMax64(*msg.CallsEnumerated)
						err := pb.Set64(*msg.CallsEnumerated)
						if err != nil {
							panic(err)
						}
						fmt.Printf("%d calls dangling.\n", *msg.FinalCallsDangling)
					} else if msg.CallsEnumerated != nil {
						err := pb.Set64(*msg.CallsEnumerated)
						if err != nil {
							panic(err)
						}
					}

				}
			}

			return c.CallsFsck(ctx, params, progressCb)
		},
	}

	flags, err := gcli.ParseV3(params)
	if err != nil {
		panic(err)
	}

	c.Flags = flags

	return c
}
