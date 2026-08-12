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
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/hashicorp/go-tfe"
	"go.uber.org/zap"
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
// (see newProjectResource). tfe.Project embeds a jsonapi.NullableAttr field that
// encoding/json cannot marshal, so the full struct can't go through the session
// store — hence this projection instead of caching *tfe.Project directly.
// Workspaces.List/ReadWithOptions request Include: WSProject so these fields
// are populated (not just the JSON:API relationship linkage's ID).
type cachedProject struct {
	ID          string
	Name        string
	Description string
	IsUnified   bool
}

// cacheWorkspacesProject is a best-effort write: a session-store failure should
// not fail the whole List page, since getWorkspaceProject has a working API
// fallback for a cache miss.
func (o *workspaceBuilder) cacheWorkspacesProject(ctx context.Context, opts resourceSdk.SyncOpAttrs, workspaces *tfe.WorkspaceList) {
	if opts.Session == nil {
		return
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
		return
	}

	if err := session.SetManyJSON(ctx, opts.Session, items, sessions.WithSyncID(opts.SyncID)); err != nil {
		ctxzap.Extract(ctx).Debug("baton-terraform-cloud: failed to cache workspace projects", zap.Error(err))
	}
}

// getWorkspaceProject may return (nil, nil) for a workspace that has no
// project — callers must guard against a nil project before dereferencing it.
func (o *workspaceBuilder) getWorkspaceProject(ctx context.Context, opts resourceSdk.SyncOpAttrs, workspaceID, parentID string) (*tfe.Project, error) {
	if opts.Session != nil {
		cached, found, err := session.GetJSON[cachedProject](ctx, opts.Session, workspaceProjectKey(workspaceID), sessions.WithSyncID(opts.SyncID))
		if err != nil {
			ctxzap.Extract(ctx).Debug("baton-terraform-cloud: failed to read cached workspace project, falling back to API", zap.Error(err))
		} else if found {
			return &tfe.Project{
				ID:          cached.ID,
				Name:        cached.Name,
				Description: cached.Description,
				IsUnified:   cached.IsUnified,
			}, nil
		}
	}

	workspace, err := o.client.Workspaces.ReadWithOptions(ctx, parentID, workspaceID, &tfe.WorkspaceReadOptions{
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
		Include:     []tfe.WSIncludeOpt{tfe.WSProject},
	})

	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to list workspaces: %w", err)
	}

	if len(workspaces.Items) == 0 {
		return nil, nil, nil
	}

	// Cache the projects for the workspaces
	o.cacheWorkspacesProject(ctx, opts, workspaces)

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
	project, err := o.getWorkspaceProject(ctx, opts, resource.Id.Resource, resource.ParentResourceId.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: failed to get workspace project: %w", err)
	}
	if project == nil {
		// Workspaces aren't required to belong to a project.
		return nil, &resourceSdk.SyncOpResults{}, nil
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
