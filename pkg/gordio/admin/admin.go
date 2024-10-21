package admin

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/spf13/cobra"
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

// Command is the users command.
func Command(cfg *config.Config) []*cobra.Command {
	userCmd := &cobra.Command{
		Use:               "users",
		Aliases:           []string{"u"},
		Short:             "administers the server",
		PersistentPreRunE: cfg.PreRunE(),
	}
	userCmd.AddCommand(addUserCommand(cfg), passwdCommand(cfg))

	return []*cobra.Command{userCmd}
}

func addUserCommand(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "adds a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.NewClient(context.Background(), cfg.DB)
			if err != nil {
				return err
			}

			username := args[0]
			isAdmin, err := cmd.Flags().GetBool("admin")
			if err != nil {
				return err
			}
			email, err := cmd.Flags().GetString("email")
			if err != nil {
				return err
			}

			return AddUser(database.CtxWithDB(context.Background(), db), username, email, isAdmin)
		},
		Args: cobra.ExactArgs(1),
	}
	c.Flags().BoolP("admin", "a", false, "is admin")
	c.Flags().StringP("email", "m", "", "email address")

	return c
}

func passwdCommand(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "passwd userid",
		Short: "changes password for a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := database.NewClient(context.Background(), cfg.DB)
			if err != nil {
				return err
			}
			username := args[0]
			return Passwd(database.CtxWithDB(context.Background(), db), username)
		},
	}

	return c
}
