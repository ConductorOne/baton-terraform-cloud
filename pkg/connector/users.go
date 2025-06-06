package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

type userBuilder struct {
	client *client.Client
}

func (o *userBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return userResourceType
}

func newUserResource(user *tfe.User, parentID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"email":             user.Email,
		"twoFactorEnabled":  user.TwoFactor.Enabled,
		"twoFactorVerified": user.TwoFactor.Verified,
	}

	if user.IsAdmin != nil {
		profile["isAdmin"] = *user.IsAdmin
	}

	name := user.Username
	if user.Username == "" {
		name = user.Email + "+invited"
	}

	return resourceSdk.NewUserResource(
		name,
		userResourceType,
		user.ID,
		[]resourceSdk.UserTraitOption{
			resourceSdk.WithUserProfile(profile),
			// last login data not available in terraform api as of 20/05/2025
		},
		resourceSdk.WithParentResourceID(parentID),
	)
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *userBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	var page int
	var err error
	if pToken.Token != "" {
		page, err = strconv.Atoi(pToken.Token)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	// https://developer.hashicorp.com/terraform/cloud-docs/api-docs/organization-memberships
	memberships, err := o.client.OrganizationMemberships.List(ctx, parentResourceID.Resource, &tfe.OrganizationMembershipListOptions{
		Include:     []tfe.OrgMembershipIncludeOpt{"user"},
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list users: %w", err)
	}

	if len(memberships.Items) == 0 {
		return nil, "", nil, nil
	}

	rv := []*v2.Resource{}
	for _, membership := range memberships.Items {
		resource, err := newUserResource(membership.User, parentResourceID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create user resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if memberships.CurrentPage < memberships.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

func (o *userBuilder) CreateAccountCapabilityDetails(ctx context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

func (o *userBuilder) CreateAccount(ctx context.Context, accountInfo *v2.AccountInfo, credentialOptions *v2.CredentialOptions) (
	connectorbuilder.CreateAccountResponse,
	[]*v2.PlaintextData,
	annotations.Annotations,
	error,
) {
	pMap := accountInfo.Profile.AsMap()
	email, ok := pMap["email"].(string)
	if !ok {
		return nil, nil, nil, fmt.Errorf("baton-terraform-cloud: email not found in profile")
	}
	orgName, ok := pMap["organizationName"].(string)
	if !ok {
		return nil, nil, nil, fmt.Errorf("baton-terraform-cloud: organizationName not found in profile")
	}
	teamNames, ok := pMap["teamNames"].([]string)
	if !ok {
		teamNames = []string{"owners"}
	}

	teams, err := o.client.Teams.List(ctx, orgName, &tfe.TeamListOptions{
		Names: teamNames,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list teams: %w", err)
	}

	if len(teams.Items) == 0 {
		return nil, nil, nil, fmt.Errorf("baton-terraform-cloud: no teams found for the given names")
	}

	orgMembership, err := o.client.OrganizationMemberships.Create(ctx, orgName, tfe.OrganizationMembershipCreateOptions{
		Email: &email,
		Teams: teams.Items,
	})

	if err != nil {
		return nil, nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create user: %w", err)
	}

	return &v2.CreateAccountResponse_ActionRequiredResult{
		Message: string(orgMembership.Status),
	}, nil, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *userBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *userBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newUserBuilder(client *client.Client) *userBuilder {
	return &userBuilder{
		client: client,
	}
}
