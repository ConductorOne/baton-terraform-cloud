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

type projectBuilder struct {
	client *client.Client
}

var permissions = []string{"read", "write", "maintain", "admin", "custom"}

func (o *projectBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return projectResourceType
}

func newProjectResource(project *tfe.Project, parentID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"description": project.Description,
		"isUnified":   project.IsUnified,
	}

	return resourceSdk.NewGroupResource(
		project.Name,
		projectResourceType,
		project.ID,
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(profile),
		},
		resourceSdk.WithParentResourceID(parentID),
	)
}

func (o *projectBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
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

	projects, err := o.client.Projects.List(ctx, parentResourceID.Resource, &tfe.ProjectListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list projects: %w", err)
	}

	if len(projects.Items) == 0 {
		return nil, "", nil, nil
	}

	rv := []*v2.Resource{}
	for _, project := range projects.Items {
		resource, err := newProjectResource(project, parentResourceID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create project resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if projects.CurrentPage < projects.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

// Entitlements always returns an empty slice for projects.
func (o *projectBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	// https://developer.hashicorp.com/terraform/cloud-docs/api-docs/project-team-access#project-team-access-levels
	rv := make([]*v2.Entitlement, 0, len(permissions))
	for _, permission := range permissions {
		rv = append(rv, entitlement.NewAssignmentEntitlement(
			resource,
			permission,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription(fmt.Sprintf("Project access level %s", permission)),
			entitlement.WithDisplayName(fmt.Sprintf("Project access level %s", permission)),
		))
	}
	return rv, "", nil, nil
}

// Grants always returns an empty slice for projects since they don't have any entitlements.
func (o *projectBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var page int
	var err error
	if pToken.Token != "" {
		page, err = strconv.Atoi(pToken.Token)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	res, err := o.client.TeamProjectAccess.List(ctx, tfe.TeamProjectAccessListOptions{
		ProjectID:   resource.Id.Resource,
		ListOptions: client.ListOptions(page),
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list project team access: %w", err)
	}

	rv := []*v2.Grant{}
	for _, item := range res.Items {
		tr, err := newTeamResource(item.Team, resource.ParentResourceId)
		if err != nil {
			return nil, "", nil, err
		}

		grantOptions := []grant.GrantOption{
			grant.WithAnnotation(&v2.GrantExpandable{
				EntitlementIds: []string{
					entitlement.NewEntitlementID(tr, teamMembership),
				},
			}),
		}

		teamResourceId, err := resourceSdk.NewResourceID(teamResourceType, item.Team.ID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create resource ID for team %v: %w", item.Team.ID, err)
		}
		rv = append(rv, grant.NewGrant(
			resource,
			string(item.Access),
			teamResourceId,
			grantOptions...,
		))
	}

	var nextPage string
	if res.CurrentPage < res.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

func newProjectBuilder(client *client.Client) *projectBuilder {
	return &projectBuilder{
		client: client,
	}
}
