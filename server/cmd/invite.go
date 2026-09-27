package cmd

import (
	"context"
	"log"

	"github.com/gregriff/vogo/server/internal/crypto"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/db"
	"github.com/spf13/cobra"
)

// inviteCmd represents the invite command.
var inviteCmd = &cobra.Command{
	Use:   "create-invite",
	Short: "Run the Vogo server",
	Args:  cobra.MaximumNArgs(0),
	Run:   generateInvite,
}

func init() {
	rootCmd.AddCommand(inviteCmd)
}

func generateInvite(_ *cobra.Command, _ []string) {
	ctx := context.Background()
	db := db.GetDB(ctx)

	inviteCode := crypto.GenerateInviteCode()
	if err := dal.AddInviteCode(ctx, db, inviteCode); err != nil {
		log.Fatalf("error creating invite code: %v", err)
	}
	log.Printf("Generated Invite Code: %s", inviteCode)
}
