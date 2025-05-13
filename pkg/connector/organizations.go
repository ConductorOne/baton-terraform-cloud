package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

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
		"defaultProject":        org.DefaultProject.Name,
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

func (o *organizationsBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *organizationsBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newOrganizationBuilder(client *client.Client) *organizationsBuilder {
	return &organizationsBuilder{
		client: client,
	}
}
