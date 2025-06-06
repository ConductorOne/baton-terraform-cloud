package connector

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

const workspaceMembership = "member"

type workspaceBuilder struct {
	client           *client.Client
	m                *sync.Mutex
	workspaceProject map[string]*tfe.Project
}

func (o *workspaceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return workspaceResourceType
}

func (o *workspaceBuilder) cacheWorkspacesProject(workspaces *tfe.WorkspaceList) {
	o.m.Lock()
	defer o.m.Unlock()

	for _, workspace := range workspaces.Items {
		if workspace.Project == nil {
			continue
		}
		o.workspaceProject[workspace.ID] = workspace.Project
	}
}

func (o *workspaceBuilder) getWorkspaceProject(ctx context.Context, workspaceID, parentID string) (*tfe.Project, error) {
	o.m.Lock()
	defer o.m.Unlock()

	project, ok := o.workspaceProject[workspaceID]
	if ok {
		return project, nil
	}

	workspace, err := o.client.Workspaces.Read(ctx, parentID, workspaceID)
	if err != nil {
		return nil, err
	}

	return workspace.Project, nil
}

func newWorkspaceResource(workspace *tfe.Workspace, parentID *v2.ResourceId) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"workingDirectory": workspace.WorkingDirectory,
		"terraformVersion": workspace.TerraformVersion,
		"runsCount":        workspace.RunsCount,
		"sourceName":       workspace.SourceName,
		"sourceURL":        workspace.SourceURL,
		"environment":      workspace.Environment,
		"allowDestroyPlan": workspace.AllowDestroyPlan,
		"autoApply":        workspace.AutoApply,
		"resourceCount":    workspace.ResourceCount,
		"executionMode":    workspace.ExecutionMode,
	}

	return resourceSdk.NewGroupResource(
		workspace.Name,
		workspaceResourceType,
		workspace.ID,
		[]resourceSdk.GroupTraitOption{
			resourceSdk.WithGroupProfile(profile),
		},
		resourceSdk.WithParentResourceID(parentID),
	)
}

func (o *workspaceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
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

	workspaces, err := o.client.Workspaces.List(ctx, parentResourceID.Resource, &tfe.WorkspaceListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list workspaces: %w", err)
	}

	if len(workspaces.Items) == 0 {
		return nil, "", nil, nil
	}

	// Cache the projects for the workspaces
	o.cacheWorkspacesProject(workspaces)

	rv := []*v2.Resource{}
	for _, workspace := range workspaces.Items {
		resource, err := newWorkspaceResource(workspace, parentResourceID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create workspace resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if workspaces.CurrentPage < workspaces.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

func (o *workspaceBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			resource,
			teamMembership,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription(fmt.Sprintf("Member of %s workspace", resource.DisplayName)),
			entitlement.WithDisplayName(fmt.Sprintf("Member of %s workspace", resource.DisplayName)),
		),
	}, "", nil, nil
}

func (o *workspaceBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	project, err := o.getWorkspaceProject(ctx, resource.Id.Resource, resource.ParentResourceId.Resource)
	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to get workspace project: %w", err)
	}

	pr, err := newProjectResource(project, resource.ParentResourceId)
	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create project resource: %w", err)
	}

	entitlementIDs := []string{}
	for _, p := range permissions {
		entitlementIDs = append(entitlementIDs, entitlement.NewEntitlementID(pr, p))
	}

	grantOptions := []grant.GrantOption{
		grant.WithAnnotation(&v2.GrantExpandable{
			EntitlementIds: entitlementIDs,
		}),
	}

	projectResourceId, err := resourceSdk.NewResourceID(projectResourceType, project.ID)
	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create resource ID for project %v: %w", project.ID, err)
	}
	rv := []*v2.Grant{
		grant.NewGrant(
			resource,
			workspaceMembership,
			projectResourceId,
			grantOptions...,
		),
	}

	return rv, "", nil, nil
}

func newWorkspaceBuilder(client *client.Client) *workspaceBuilder {
	return &workspaceBuilder{
		client:           client,
		m:                &sync.Mutex{},
		workspaceProject: make(map[string]*tfe.Project),
	}
}
