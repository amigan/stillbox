package admin

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"dynatron.me/x/stillbox/pkg/gordio/config"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/jackc/pgx/v5/pgtype"
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

func AddUser(ctx context.Context, cfg *config.Config, username, email string, isAdmin bool) error {
	if username == "" || email == "" {
		return ErrInvalidArguments
	}

	db, err := database.NewClient(cfg.DB)
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

	_, err = db.CreateUser(context.Background(), database.CreateUserParams{
		Username: username,
		Password: string(hashpw),
		Email:    email,
		IsAdmin:  pgtype.Bool{Bool: isAdmin, Valid: true},
	})

	return err
}

func Passwd(ctx context.Context, cfg *config.Config, username string) error {
	if username == "" {
		return ErrInvalidArguments
	}

	db, err := database.NewClient(cfg.DB)
	if err != nil {
		return err
	}

	_, err = db.GetUserByUsername(ctx, username)
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

	return database.New(db).UpdatePassword(context.Background(), username, string(hashpw))
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return string(pw), err
}

func Command(cfg *config.Config) []*cobra.Command {
	userCmd := &cobra.Command{
		Use:     "users",
		Aliases: []string{"u"},
		Short:   "administers the server",
	}
	userCmd.AddCommand(addUserCommand(cfg), passwdCommand(cfg))

	return []*cobra.Command{userCmd}
}

func addUserCommand(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "adds a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			isAdmin, err := cmd.Flags().GetBool("admin")
			if err != nil {
				return err
			}
			email, err := cmd.Flags().GetString("email")
			if err != nil {
				return err
			}

			return AddUser(context.Background(), cfg, username, email, isAdmin)
		},
		Args: cobra.ExactArgs(1),
	}
	c.Flags().BoolP("admin", "a", false, "is admin")
	c.Flags().StringP("email", "m", "", "email address")

	return c
}

func passwdCommand(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "passwd",
		Short: "changes password for a user",
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			return Passwd(context.Background(), cfg, username)
		},
		Args: cobra.ExactArgs(1),
	}

	return c
}
