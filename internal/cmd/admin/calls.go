package admin

import (
	"context"
	"fmt"
	"os"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/calls/callstore"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/rest/client"
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
		},
	}

	return callsCmd
}

type progresser struct {
	total int64
	pb    *progressbar.ProgressBar
}

func (p *progresser) textCb(msg client.ProgressMsg) {
	switch {
	case msg.Completed != nil:
		fmt.Printf("%d calls (%d%%) done...\n", *msg.Completed, int((float32(*msg.Completed)/float32(p.total))*100))
	case msg.Total != nil:
		p.total = *msg.Total
	case msg.Final != nil:
		fmt.Printf("Finished %d calls\n", *msg.Final)
	}
}

func (p *progresser) ttyCb(msg client.ProgressMsg) {
	switch {
	case msg.Completed != nil:
		if p.pb == nil {
			break
		}
		err := p.pb.Set64(*msg.Completed)
		if err != nil {
			panic(err)
		}
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
				fmt.Fprint(os.Stderr, "\n")
			}),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionFullWidth(),
			progressbar.OptionSetRenderBlankState(true),
		)
	case msg.Final != nil:
		if p.pb != nil {
			err := p.pb.Set64(*msg.Final)
			if err != nil {
				panic(err)
			}
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
		Description: "checks partition interval matches whatever is specified in the config",
		UsageText:   "stillbox admin database partitioning",
		Flags:       []cli.Flag{},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cfg.Server.AdminSocket == nil {
				return fmt.Errorf("no admin socket configured")
			}

			prog := progresser{}
			var progressCb func(msg client.ProgressMsg)
			if isatty.IsTerminal(os.Stdout.Fd()) {
				progressCb = prog.ttyCb
			} else {
				progressCb = prog.textCb
			}

			c, err := client.New(client.UnixSocket(*cfg.Server.AdminSocket))
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
