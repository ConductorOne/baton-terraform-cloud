package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

const orgMembership = "member"

var _ connectorbuilder.ResourceSyncerV2 = (*organizationsBuilder)(nil)

type organizationsBuilder struct {
	client *client.Client
}

func (o *organizationsBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return organizationResourceType
}

func newOrganizationResource(org *tfe.Organization) (*v2.Resource, error) {
	profile := map[string]interface{}{
		emailProfileKey:         org.Email,
		"costEstimationEnabled": org.CostEstimationEnabled,
		"twoFactorConformant":   org.TwoFactorConformant,
	}
	return resourceSdk.NewGroupResource(
		org.Name,
		organizationResourceType,
		org.Name, // yes the name is the id: https://developer.hashicorp.com/terraform/cloud-docs/api-docs/organizations#show-an-organization
		nil,
		resourceSdk.WithResourceProfile(profile),
		resourceSdk.WithAnnotation(
			&v2.ChildResourceType{ResourceTypeId: userResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: teamResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: projectResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: workspaceResourceType.Id},
			&v2.ChildResourceType{ResourceTypeId: agentTokenResourceType.Id},
		),
	)
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *organizationsBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts resourceSdk.SyncOpAttrs) ([]*v2.Resource, *resourceSdk.SyncOpResults, error) {
	var page int
	var err error
	if opts.PageToken.Token != "" {
		page, err = strconv.Atoi(opts.PageToken.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	orgs, err := o.client.Organizations.List(ctx, &tfe.OrganizationListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list organizations: %w", err)
	}

	rv := make([]*v2.Resource, 0, len(orgs.Items))
	for _, org := range orgs.Items {
		resource, err := newOrganizationResource(org)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create organization resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if orgs.CurrentPage < orgs.TotalPages {
		nextPage = strconv.Itoa(orgs.CurrentPage + 1)
	}

	return rv, &resourceSdk.SyncOpResults{NextPageToken: nextPage}, nil
}

func (o *organizationsBuilder) Entitlements(ctx context.Context, resource *v2.Resource, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *organizationsBuilder) StaticEntitlements(ctx context.Context, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			nil,
			teamMembership,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription("Member of organization team"),
			entitlement.WithDisplayName("Member of organization team"),
		),
	}, nil, nil
}

func (o *organizationsBuilder) Grants(ctx context.Context, resource *v2.Resource, opts resourceSdk.SyncOpAttrs) ([]*v2.Grant, *resourceSdk.SyncOpResults, error) {
	var page int
	var err error
	if opts.PageToken.Token != "" {
		page, err = strconv.Atoi(opts.PageToken.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	// https://developer.hashicorp.com/terraform/cloud-docs/api-docs/organization-memberships
	memberships, err := o.client.OrganizationMemberships.List(ctx, resource.Id.Resource, &tfe.OrganizationMembershipListOptions{
		Include:     []tfe.OrgMembershipIncludeOpt{userResourceTypeID},
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list users: %w", err)
	}

	rv := []*v2.Grant{}
	for _, membership := range memberships.Items {
		principalID, err := resourceSdk.NewResourceID(userResourceType, membership.User.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create user resource ID: %w", err)
		}
		rv = append(rv, grant.NewGrant(
			resource,
			orgMembership,
			principalID,
		))
	}

	var nextPage string
	if memberships.CurrentPage < memberships.TotalPages {
		nextPage = strconv.Itoa(memberships.CurrentPage + 1)
	}

	return rv, &resourceSdk.SyncOpResults{NextPageToken: nextPage}, nil
}

func (o *organizationsBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	return nil, nil
}

// principalEmail resolves the email address the organization membership API needs
// to identify a principal.
//
// It reads the user trait's primary email first. baton-sdk v0.20.1 deprecated the
// profile on the user trait in favour of a resource-level profile, but left the
// trait's emails alone, so this is both the non-deprecated and the semantically
// correct field — and it does not depend on the platform hydrating the newer
// resource-level profile onto the principal it sends us. The resource-level
// profile is the fallback.
func principalEmail(principal *v2.Resource) (string, error) {
	if userTrait, err := resourceSdk.GetUserTrait(principal); err == nil {
		var firstAddress string
		for _, email := range userTrait.GetEmails() {
			address := email.GetAddress()
			if address == "" {
				continue
			}
			if email.GetIsPrimary() {
				return address, nil
			}
			if firstAddress == "" {
				firstAddress = address
			}
		}
		if firstAddress != "" {
			return firstAddress, nil
		}
	}

	if address, ok := principal.GetProfile().AsMap()[emailProfileKey].(string); ok && address != "" {
		return address, nil
	}

	return "", fmt.Errorf("baton-terraform-cloud: failed to resolve email for principal %s", principal.GetId().GetResource())
}

func (o *organizationsBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	entitlement := grant.Entitlement
	orgName := entitlement.Resource.Id.Resource

	email, err := principalEmail(grant.Principal)
	if err != nil {
		return nil, err
	}

	orgMemberships, err := o.client.OrganizationMemberships.List(ctx, orgName, &tfe.OrganizationMembershipListOptions{
		Emails: []string{email},
	})
	if err != nil {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to list organization memberships: %w", err)
	}

	if len(orgMemberships.Items) == 0 {
		return annotations.New(&v2.GrantAlreadyRevoked{}), nil
	}

	orgMembershipId := orgMemberships.Items[0].ID
	err = o.client.OrganizationMemberships.Delete(ctx, orgMembershipId)
	if err != nil {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to remove user from organization: %w", err)
	}
	return nil, nil
}

func newOrganizationBuilder(client *client.Client) *organizationsBuilder {
	return &organizationsBuilder{
		client: client,
	}
}
