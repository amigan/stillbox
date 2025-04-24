package admin

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/bcrypt"
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

// AddUser adds a new user to the database. It asks for the password on the terminal.
func AddUser(ctx context.Context, username, email string, isAdmin bool) error {
	if username == "" || email == "" {
		return ErrInvalidArguments
	}

	db := database.FromCtx(ctx)

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

	hashpw, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.CreateUser(context.Background(), database.CreateUserParams{
		Username: username,
		Password: string(hashpw),
		Email:    email,
		IsAdmin:  isAdmin,
	})

	return err
}

// Passwd changes a user's password. It asks for the password on the terminal.
func Passwd(ctx context.Context, username string) error {
	if username == "" {
		return ErrInvalidArguments
	}

	db := database.FromCtx(ctx)

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

	hashpw, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return db.UpdatePassword(context.Background(), username, string(hashpw))
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
		Subcommands: []*cli.Command{
			addUserCommand(c),
			passwdCommand(c),
		},
	}

	return userCmd
}

func addUserCommand(cfg *config.Config) *cli.Command {
	c := &cli.Command{
		Name:        "add",
		Description: "adds a user",
		UsageText:   "stillbox users add [-a] [-m email] [username]",
		Args:        true,
		Action: func(ctx *cli.Context) error {
			if ctx.Args().Len() != 1 {
				return errors.New(ctx.Command.Usage)
			}

			db, err := database.NewClient(context.Background(), cfg.DB)
			if err != nil {
				return err
			}

			username := ctx.Args().Get(0)
			isAdmin := ctx.Bool("admin")
			email := ctx.String("email")

			return AddUser(database.CtxWithDB(context.Background(), db), username, email, isAdmin)
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
		},
	}

	return c
}

func passwdCommand(cfg *config.Config) *cli.Command {
	c := &cli.Command{
		Name:      "passwd",
		Usage:     "changes password for a user",
		UsageText: "stillbox users passwd [username]",
		Args:      true,
		Action: func(ctx *cli.Context) error {
			if ctx.Args().Len() != 1 {
				return errors.New(ctx.Command.Usage)
			}

			db, err := database.NewClient(context.Background(), cfg.DB)
			if err != nil {
				return err
			}
			username := ctx.Args().Get(0)

			err = Passwd(database.CtxWithDB(context.Background(), db), username)
			if err != nil {
				return err
			}

			fmt.Println("Password successfully changed.", "Make sure to send SIGHUP to any running stillbox processes to invalidate any prior sessions!")

			return nil
		},
	}

	return c
}
