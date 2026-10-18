package contract

import (
	"os"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/seed"
)

// TestSignInFixtureAccounts signs in as the three fixture users. Each call
// uses a new cookie jar. Stored session rows are not read.
func TestSignInFixtureAccounts(t *testing.T) {
	web := os.Getenv("CONTRACT_WEB_URL")
	if web == "" {
		t.Fatal("missing CONTRACT_WEB_URL")
	}
	ctx := testCtx(t)
	accounts := []struct {
		user string
		pass string
	}{
		{seed.AdminUsername, seed.AdminPassword},
		{seed.LearnerUsername, seed.LearnerPassword},
		{seed.LearnerOffUsername, seed.LearnerOffPassword},
	}
	seen := map[string]string{}
	for _, acct := range accounts {
		cookie, err := seed.SignIn(ctx, web, acct.user, acct.pass)
		if err != nil {
			t.Fatalf("%s: %v", acct.user, err)
		}
		if prev, ok := seen[cookie]; ok {
			t.Fatalf("session cookie for %s was already issued to %s", acct.user, prev)
		}
		seen[cookie] = acct.user
	}
	again, err := seed.SignIn(ctx, web, seed.AdminUsername, seed.AdminPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := seen[again]; ok {
		t.Fatal("second admin login reused a session cookie")
	}
}
