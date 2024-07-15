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

func addUser(cfg *config.Config, username, email string, isAdmin bool) error {
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

	_, err = database.New(db).CreateUser(context.Background(), database.CreateUserParams{
		Username: username,
		Password: string(hashpw),
		Email:    email,
		IsAdmin:  pgtype.Bool{Bool: isAdmin, Valid: true},
	})

	return err
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
	userCmd.AddCommand(addUserCommand(cfg))

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

			return addUser(cfg, username, email, isAdmin)
		},
		Args: cobra.ExactArgs(1),
	}
	c.Flags().BoolP("admin", "a", false, "is admin")
	c.Flags().StringP("email", "m", "", "email address")

	return c
}
