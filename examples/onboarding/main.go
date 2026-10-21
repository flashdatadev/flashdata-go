// This example demonstrates machine-side onboarding primitives. A future CLI
// owns secure storage and fresh-process installation; the SDK owns no vault.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	flashdata "github.com/flashdatadev/flashdata-go"
)

func main() {
	action := flag.String("action", "capabilities", "capabilities, status, claim, identity, or account-identity")
	id := flag.String("connection", "", "connection ID from durably stored onboarding state")
	flag.Parse()
	client, err := flashdata.New(flashdata.Options{APIKey: os.Getenv("FLASHDATA_API_KEY"), DataOrigin: os.Getenv("FLASHDATA_DATA_ORIGIN"), ManagementOrigin: os.Getenv("FLASHDATA_MANAGEMENT_ORIGIN")})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var result any
	switch *action {
	case "capabilities":
		result, err = client.Onboarding.Capabilities(ctx)
	case "status":
		result, err = client.Onboarding.Status(ctx, *id, os.Getenv("FLASHDATA_HANDOFF_PROOF"))
	case "identity":
		result, err = client.Onboarding.Identity(ctx, *id, os.Getenv("FLASHDATA_API_KEY"))
	case "account-identity":
		result, err = client.Account.Identity(ctx)
	case "claim":
		claim, callErr := client.Onboarding.CreateClaim(ctx, *id, os.Getenv("FLASHDATA_SETUP_CREDENTIAL"))
		err = callErr
		if err == nil {
			link, linkErr := flashdata.ClaimURL("https://flashdata.dev", claim)
			err = linkErr
			if err == nil {
				result = map[string]string{"url": link, "confirmationCode": claim.ConfirmationCode, "expiresAt": claim.ExpiresAt}
			}
		}
	default:
		err = fmt.Errorf("unknown action; registration and handoff require a durable credential store; see docs/onboarding.md")
	}
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatal(err)
	}
}
