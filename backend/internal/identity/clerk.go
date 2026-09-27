package identity

import (
	"context"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2/user"
)

type ClerkProfileProvider struct{}

func (ClerkProfileProvider) GetProfile(ctx context.Context, subject string) (Profile, error) {
	clerkUser, err := user.Get(ctx, subject)
	if err != nil {
		return Profile{}, err
	}
	if clerkUser == nil || clerkUser.ID != subject || clerkUser.Banned || clerkUser.Locked || clerkUser.PrimaryEmailAddressID == nil {
		return Profile{}, ErrIncompleteProfile
	}

	var email string
	for _, address := range clerkUser.EmailAddresses {
		if address.ID == *clerkUser.PrimaryEmailAddressID && address.Verification != nil && address.Verification.Status == "verified" {
			email = address.EmailAddress
			break
		}
	}
	if email == "" {
		return Profile{}, ErrIncompleteProfile
	}

	nameParts := make([]string, 0, 2)
	if clerkUser.FirstName != nil {
		nameParts = append(nameParts, *clerkUser.FirstName)
	}
	if clerkUser.LastName != nil {
		nameParts = append(nameParts, *clerkUser.LastName)
	}
	name := strings.TrimSpace(strings.Join(nameParts, " "))
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	return Profile{Email: email, DisplayName: name}, nil
}
