package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/users"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

const (
	PromptPassword = "Password: "
	PromptAgain    = "Again: "
)

var (
	ErrDontMatch        = errors.New("passwords do not match")
	ErrInvalidArguments = errors.New("invalid arguments")
)

// AddUser adds a new user to the database. It asks for the password on the terminal, or reads from stdin if not a terminal.
func AddUser(ctx context.Context, username, realName, email string, isAdmin bool) error {
	if username == "" || email == "" {
		return ErrInvalidArguments
	}

	ust := users.FromCtx(ctx)

	var err error
	var pw string

	if isatty.IsTerminal(os.Stdin.Fd()) {
		pw, err = readPassword(PromptPassword)
		if err != nil {
			return err
		}

		pwAgain, err := readPassword(PromptAgain)
		if err != nil {
			return err
		}

		if pwAgain != pw {
			return ErrDontMatch
		}
	} else {
		pwb, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		pw = string(pwb)
		if nlIdx := strings.IndexRune(pw, '\n'); nlIdx > -1 {
			pw = pw[:nlIdx]
		}
	}

	if pw == "" {
		return ErrInvalidArguments
	}

	var realNameP *string
	if realName != "" {
		realNameP = &realName
	}

	var roles []string
	if isAdmin {
		roles = []string{entities.RoleAdmin}
	}

	user, err := ust.AddUser(ctx, &users.User{
		Username: username,
		Password: pw,
		RealName: realNameP,
		Email:    email,
		Roles:    roles,
	})
	if err != nil {
		return err
	}

	log.Info().Int("uid", user.ID.Int()).Str("username", user.Username).Msg("added user")

	return nil
}

// Passwd changes a user's password. It asks for the password on the terminal.
func Passwd(ctx context.Context, username string) error {
	if username == "" {
		return ErrInvalidArguments
	}

	db := database.FromCtx(ctx)
	ust := users.FromCtx(ctx)

	_, err := db.GetUserByUsername(ctx, username)
	if err != nil && database.IsNoRows(err) {
		return fmt.Errorf("no such user %s", username)
	}

	if err != nil {
		return err
	}

	pw, err := readPassword(PromptPassword)
	if err != nil {
		return err
	}

	pwAgain, err := readPassword(PromptAgain)
	if err != nil {
		return err
	}

	if pwAgain != pw {
		return ErrDontMatch
	}

	if pw == "" {
		return ErrInvalidArguments
	}

	return ust.ChangePassword(ctx, username, pw)
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return string(pw), err
}

// UsersCommand is the users command.
func UsersCommand(cfg *config.Configuration) *cli.Command {
	c := &cfg.Config
	userCmd := &cli.Command{
		Name:    "users",
		Aliases: []string{"u"},
		Usage:   "administers users",
		Commands: []*cli.Command{
			addUserCommand(c),
			passwdCommand(c),
		},
	}

	return userCmd
}

func addUserCommand(cfg *config.Config) *cli.Command {
	c := &cli.Command{
		Name:        "add",
		Description: "Adds a user to the database using the provided options.",
		UsageText:   "stillbox users add [-a] [-m email] [username]",
		ArgsUsage:   "username",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return errors.New(cmd.UsageText)
			}

			db, err := database.NewClient(ctx, cfg.DB)
			if err != nil {
				return err
			}

			ctx = database.CtxWithDB(ctx, db)
			ctx = users.CtxWithStore(ctx, users.NewStore(db))

			username := cmd.Args().Get(0)
			isAdmin := cmd.Bool("admin")
			email := cmd.String("email")
			realName := cmd.String("real-name")

			return AddUser(ctx, username, realName, email, isAdmin)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "admin",
				Aliases: []string{"a"},
				Value:   false,
				Usage:   "user is an admin",
			},
			&cli.StringFlag{
				Name:    "email",
				Usage:   "email address",
				Aliases: []string{"m"},
			},
			&cli.StringFlag{
				Name:    "real-name",
				Usage:   "real name",
				Aliases: []string{"N"},
			},
		},
	}

	return c
}

func passwdCommand(cfg *config.Config) *cli.Command {
	c := &cli.Command{
		Name:  "passwd",
		Usage: "stillbox admin users passwd [username]",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return errors.New(cmd.Usage)
			}

			db, err := database.NewClient(ctx, cfg.DB)
			if err != nil {
				return err
			}
			username := cmd.Args().Get(0)

			err = Passwd(database.CtxWithDB(ctx, db), username)
			if err != nil {
				return err
			}

			fmt.Println("Password successfully changed.", "Make sure to send SIGHUP to any running stillbox processes to invalidate any prior sessions!")

			return nil
		},
	}

	return c
}
