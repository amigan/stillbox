package admin

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/database"
	"github.com/urfave/cli/v3"
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
func AddUser(ctx context.Context, username, realName, email string, isAdmin bool) error {
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

	var realNameP *string
	if realName != "" {
		realNameP = &realName
	}

	var roles []string
	if isAdmin {
		roles = []string{entities.RoleAdmin}
	}

	_, err = db.CreateUser(ctx, database.CreateUserParams{
		Username: username,
		Password: string(hashpw),
		RealName: realNameP,
		Email:    email,
		Roles:    roles,
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

	return db.UpdatePassword(ctx, username, string(hashpw))
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
		Description: "adds a user",
		UsageText:   "stillbox users add [-a] [-m email] [username]",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() != 1 {
				return errors.New(cmd.Usage)
			}

			db, err := database.NewClient(ctx, cfg.DB)
			if err != nil {
				return err
			}

			username := cmd.Args().Get(0)
			isAdmin := cmd.Bool("admin")
			email := cmd.String("email")
			realName := cmd.String("real-name")

			return AddUser(database.CtxWithDB(ctx, db), username, realName, email, isAdmin)
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
