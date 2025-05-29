package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

const orgMembership = "member"

type organizationsBuilder struct {
	client *client.Client
}

func (o *organizationsBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return organizationResourceType
}

func newOrganizationResource(org *tfe.Organization) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"email":                 org.Email,
		"costEstimationEnabled": org.CostEstimationEnabled,
		"twoFactorConformant":   org.TwoFactorConformant,
	}
	return resourceSdk.NewGroupResource(
		org.Name,
		organizationResourceType,
		org.Name, // yes the name is the id: https://developer.hashicorp.com/terraform/cloud-docs/api-docs/organizations#show-an-organization
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(profile),
		},
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
func (o *organizationsBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var page int
	var err error
	if pToken.Token != "" {
		page, err = strconv.Atoi(pToken.Token)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	orgs, err := o.client.Organizations.List(ctx, &tfe.OrganizationListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list organizations: %w", err)
	}

	if len(orgs.Items) == 0 {
		return nil, "", nil, nil
	}

	rv := make([]*v2.Resource, 0, len(orgs.Items))
	for _, org := range orgs.Items {
		resource, err := newOrganizationResource(org)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create organization resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if orgs.CurrentPage < orgs.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

func (o *organizationsBuilder) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			resource,
			teamMembership,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription(fmt.Sprintf("Member of %s team", resource.DisplayName)),
			entitlement.WithDisplayName(fmt.Sprintf("Member of %s team", resource.DisplayName)),
		),
	}, "", nil, nil
}

func (o *organizationsBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var page int
	var err error
	if pToken.Token != "" {
		page, err = strconv.Atoi(pToken.Token)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	// https://developer.hashicorp.com/terraform/cloud-docs/api-docs/organization-memberships
	memberships, err := o.client.OrganizationMemberships.List(ctx, resource.Id.Resource, &tfe.OrganizationMembershipListOptions{
		Include:     []tfe.OrgMembershipIncludeOpt{"user"},
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list users: %w", err)
	}

	if len(memberships.Items) == 0 {
		return nil, "", nil, nil
	}

	rv := []*v2.Grant{}
	for _, membership := range memberships.Items {
		principalID, err := resourceSdk.NewResourceID(userResourceType, membership.User.ID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create user resource ID: %w", err)
		}
		rv = append(rv, grant.NewGrant(
			resource,
			orgMembership,
			principalID,
		))
	}

	var nextPage string
	if memberships.CurrentPage < memberships.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

func (o *organizationsBuilder) Grant(ctx context.Context, principal *v2.Resource, entitlement *v2.Entitlement) (annotations.Annotations, error) {
	return nil, nil
}

func (o *organizationsBuilder) Revoke(ctx context.Context, grant *v2.Grant) (annotations.Annotations, error) {
	entitlement := grant.Entitlement
	orgName := entitlement.Resource.Id.Resource

	userTrait, err := resourceSdk.GetUserTrait(grant.Principal)
	if err != nil {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to get user trait: %w", err)
	}

	profile := userTrait.GetProfile().AsMap()
	email, ok := profile["email"].(string)
	if !ok {
		return nil, fmt.Errorf("baton-terraform-cloud: failed to get email from user trait")
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
