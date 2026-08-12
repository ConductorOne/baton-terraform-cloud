package connector

import (
	"context"
	"fmt"
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/session"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resourceSdk "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
	"github.com/hashicorp/go-tfe"
)

const (
	workspaceMembership     = "member"
	workspaceProjectKeyBase = "workspace-project"
)

var _ connectorbuilder.ResourceSyncerV2 = (*workspaceBuilder)(nil)

type workspaceBuilder struct {
	client *client.Client
}

func (o *workspaceBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return workspaceResourceType
}

// workspaceProjectKey returns the session store key for a workspace's cached project.
// Caching goes through the session store (rather than an in-process map) because a
// container/Lambda deployment may run List and Grants for the same sync in separate
// invocations that don't share memory.
func workspaceProjectKey(workspaceID string) string {
	return fmt.Sprintf("%s/%s", workspaceProjectKeyBase, workspaceID)
}

// cachedProject holds only the *tfe.Project fields the connector actually reads
// (see newProjectResource). tfe.Project embeds jsonapi.NullableAttr fields that
// encoding/json cannot marshal, so the full struct can't go through the session store.
type cachedProject struct {
	ID          string
	Name        string
	Description string
	IsUnified   bool
}

func (o *workspaceBuilder) cacheWorkspacesProject(ctx context.Context, opts resourceSdk.SyncOpAttrs, workspaces *tfe.WorkspaceList) error {
	if opts.Session == nil {
		return nil
	}

	items := make(map[string]cachedProject)
	for _, workspace := range workspaces.Items {
		if workspace.Project == nil {
			continue
		}
		items[workspaceProjectKey(workspace.ID)] = cachedProject{
			ID:          workspace.Project.ID,
			Name:        workspace.Project.Name,
			Description: workspace.Project.Description,
			IsUnified:   workspace.Project.IsUnified,
		}
	}
	if len(items) == 0 {
		return nil
	}

	return session.SetManyJSON(ctx, opts.Session, items, sessions.WithSyncID(opts.SyncID))
}

func (o *workspaceBuilder) getWorkspaceProject(ctx context.Context, opts resourceSdk.SyncOpAttrs, workspaceID, parentID string) (*tfe.Project, error) {
	if opts.Session != nil {
		cached, found, err := session.GetJSON[cachedProject](ctx, opts.Session, workspaceProjectKey(workspaceID), sessions.WithSyncID(opts.SyncID))
		if err != nil {
			return nil, fmt.Errorf("baton-terraform-cloud: failed to read cached workspace project: %w", err)
		}
		if found {
			return &tfe.Project{
				ID:          cached.ID,
				Name:        cached.Name,
				Description: cached.Description,
				IsUnified:   cached.IsUnified,
			}, nil
		}
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
		nil,
		resourceSdk.WithResourceProfile(profile),
		resourceSdk.WithParentResourceID(parentID),
	)
}

func (o *workspaceBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, opts resourceSdk.SyncOpAttrs) ([]*v2.Resource, *resourceSdk.SyncOpResults, error) {
	if parentResourceID == nil {
		return nil, &resourceSdk.SyncOpResults{}, nil
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

	if len(workspaces.Items) == 0 {
		return nil, &resourceSdk.SyncOpResults{}, nil
	}

	// Cache the projects for the workspaces
	if err := o.cacheWorkspacesProject(ctx, opts, workspaces); err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to cache workspace projects: %w", err)
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
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, &resourceSdk.SyncOpResults{NextPageToken: nextPage}, nil
}

func (o *workspaceBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ resourceSdk.SyncOpAttrs) ([]*v2.Entitlement, *resourceSdk.SyncOpResults, error) {
	return []*v2.Entitlement{
		entitlement.NewAssignmentEntitlement(
			resource,
			teamMembership,
			entitlement.WithGrantableTo(userResourceType),
			entitlement.WithDescription(fmt.Sprintf("Member of %s workspace", resource.DisplayName)),
			entitlement.WithDisplayName(fmt.Sprintf("Member of %s workspace", resource.DisplayName)),
		),
	}, &resourceSdk.SyncOpResults{}, nil
}

func (o *workspaceBuilder) Grants(ctx context.Context, resource *v2.Resource, opts resourceSdk.SyncOpAttrs) ([]*v2.Grant, *resourceSdk.SyncOpResults, error) {
	project, err := o.getWorkspaceProject(ctx, opts, resource.Id.Resource, resource.ParentResourceId.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to get workspace project: %w", err)
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

	return rv, &resourceSdk.SyncOpResults{}, nil
}

func newWorkspaceBuilder(client *client.Client) *workspaceBuilder {
	return &workspaceBuilder{
		client: client,
	}
}
