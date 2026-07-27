package connector

import (
	"testing"

	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
)

// TestPrincipalEmail covers each way a principal can carry its email address, so a
// revoke keeps resolving the organization member regardless of whether the platform
// hydrates the user trait's emails, the resource-level profile, or both.
//
// baton-sdk v0.20.1 moved the profile off the user trait onto the resource; the
// trait's emails were untouched. Revoke therefore reads the trait first and treats
// the resource-level profile as a fallback.
func TestPrincipalEmail(t *testing.T) {
	tests := []struct {
		name             string
		traitOptions     []resourceSdk.UserTraitOption
		profile          map[string]interface{}
		expectedEmail    string
		expectedErrorMsg bool
	}{
		{
			// A resource built the way newUserResource builds it.
			name:          "primary trait email and resource profile",
			traitOptions:  []resourceSdk.UserTraitOption{resourceSdk.WithEmail("primary@example.com", true)},
			profile:       map[string]interface{}{emailProfileKey: "profile@example.com"},
			expectedEmail: "primary@example.com",
		},
		{
			// The platform sent the resource-level profile but no trait emails.
			name:          "resource profile only",
			profile:       map[string]interface{}{emailProfileKey: "profile@example.com"},
			expectedEmail: "profile@example.com",
		},
		{
			// The platform sent trait emails but not the newer resource-level profile.
			name:          "trait email only",
			traitOptions:  []resourceSdk.UserTraitOption{resourceSdk.WithEmail("trait@example.com", true)},
			expectedEmail: "trait@example.com",
		},
		{
			name:          "non-primary trait email is still usable",
			traitOptions:  []resourceSdk.UserTraitOption{resourceSdk.WithEmail("secondary@example.com", false)},
			expectedEmail: "secondary@example.com",
		},
		{
			name:             "no email anywhere",
			expectedErrorMsg: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resourceOptions := []resourceSdk.ResourceOption{}
			if test.profile != nil {
				resourceOptions = append(resourceOptions, resourceSdk.WithResourceProfile(test.profile))
			}

			principal, err := resourceSdk.NewUserResource(
				"Test User",
				userResourceType,
				"user-1",
				test.traitOptions,
				resourceOptions...,
			)
			if err != nil {
				t.Fatalf("failed to build principal: %v", err)
			}

			email, err := principalEmail(principal)

			if test.expectedErrorMsg {
				if err == nil {
					t.Fatalf("expected an error, got email %q", email)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if email != test.expectedEmail {
				t.Errorf("expected email %q, got %q", test.expectedEmail, email)
			}
		})
	}
}
