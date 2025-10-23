package cmd

import (
	"log"

	"github.com/gregriff/vogo/server/internal/crypto"
	"github.com/gregriff/vogo/server/internal/dal"
	"github.com/gregriff/vogo/server/internal/db"
	"github.com/spf13/cobra"
	// _ "net/http/pprof".
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
	inviteCode := crypto.GenerateInviteCode()
	db := db.GetDB()
	if err := dal.AddInviteCode(db, inviteCode); err != nil {
		log.Fatalf("error creating invite code: %v", err)
	}
	log.Printf("Generated Invite Code: %s", inviteCode)
}
