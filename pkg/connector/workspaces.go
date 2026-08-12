package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

const workspaceMembership = "member"

var _ connectorbuilder.ResourceSyncerV2 = (*workspaceBuilder)(nil)

type workspaceBuilder struct {
	client *client.Client
}

func (o *workspaceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return workspaceResourceType
}

// getWorkspaceProject may return (nil, nil) for a workspace that has no
// project — callers must guard against a nil project before dereferencing it.
//
// Uses ReadByIDWithOptions, not ReadWithOptions: resource.Id.Resource is the
// workspace's ID (e.g. "ws-..."), and ReadWithOptions' third argument is the
// workspace's *name*, not its ID — passing the ID there 404s.
func (o *workspaceBuilder) getWorkspaceProject(ctx context.Context, workspaceID string) (*tfe.Project, error) {
	workspace, err := o.client.Workspaces.ReadByIDWithOptions(ctx, workspaceID, &tfe.WorkspaceReadOptions{
		Include: []tfe.WSIncludeOpt{tfe.WSProject},
	})
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
		nil,
		resourceSdk.WithResourceProfile(profile),
		resourceSdk.WithParentResourceID(parentID),
	)
}

func (o *workspaceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts resourceSdk.SyncOpAttrs) ([]*v2.Resource, *resourceSdk.SyncOpResults, error) {
	if parentResourceID == nil {
		return nil, nil, nil
	}

	var page int
	var err error
	if opts.PageToken.Token != "" {
		page, err = strconv.Atoi(opts.PageToken.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to parse page token: %w", err)
		}
	}

	workspaces, err := o.client.Workspaces.List(ctx, parentResourceID.Resource, &tfe.WorkspaceListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list workspaces: %w", err)
	}

	rv := []*v2.Resource{}
	for _, workspace := range workspaces.Items {
		resource, err := newWorkspaceResource(workspace, parentResourceID)
		if err != nil {
			return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create workspace resource: %w", err)
		}
		rv = append(rv, resource)
	}

	var nextPage string
	if workspaces.CurrentPage < workspaces.TotalPages {
		nextPage = strconv.Itoa(workspaces.CurrentPage + 1)
	}

	return rv, &resourceSdk.SyncOpResults{NextPageToken: nextPage}, nil
}

func (o *workspaceBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return nil, nil, nil
}

func (o *workspaceBuilder) StaticEntitlements(_ context.Context, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			nil,
			teamMembership,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription("Member of workspace"),
			entitlement.WithDisplayName("Member of workspace"),
		),
	}, nil, nil
}

func (o *workspaceBuilder) Grants(ctx context.Context, resource *v2.Resource, opts resourceSdk.SyncOpAttrs) ([]*v2.Grant, *resourceSdk.SyncOpResults, error) {
	project, err := o.getWorkspaceProject(ctx, resource.Id.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to get workspace project: %w", err)
	}
	if project == nil {
		// Workspaces aren't required to belong to a project.
		return nil, nil, nil
	}

	pr, err := newProjectResource(project, resource.ParentResourceId)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create project resource: %w", err)
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
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to create resource ID for project %v: %w", project.ID, err)
	}
	rv := []*v2.Grant{
		grant.NewGrant(
			resource,
			workspaceMembership,
			projectResourceId,
			grantOptions...,
		),
	}

	return rv, nil, nil
}

func newWorkspaceBuilder(client *client.Client) *workspaceBuilder {
	return &workspaceBuilder{
		client: client,
	}
}
