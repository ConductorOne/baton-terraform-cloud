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

type agentTokenBuilder struct {
	client *client.Client
}

func (o *agentTokenBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return agentTokenResourceType
}

func newAgentTokenResource(agentToken *tfe.AgentToken, parentID *v2.ResourceId) (*v2.Resource, error) {
	return resourceSdk.NewSecretResource(
		agentToken.Description,
		agentTokenResourceType,
		agentToken.ID,
		[]resourceSdk.SecretTraitOption{
			resourceSdk.WithSecretCreatedAt(agentToken.CreatedAt),
			resourceSdk.WithSecretLastUsedAt(agentToken.LastUsedAt),
		},
		resourceSdk.WithParentResourceID(parentID),
	)
}

// List returns all the agentTokens from the database as resource objects.
// AgentTokens include a AgentTokenTrait because they are the 'shape' of a standard agentToken.
func (o *agentTokenBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
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

	agentPools, err := o.client.AgentPools.List(ctx, parentResourceID.Resource, &tfe.AgentPoolListOptions{
		ListOptions: client.ListOptions(page),
	})

	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list agent pools: %w", err)
	}

	if len(agentPools.Items) == 0 {
		return nil, "", nil, nil
	}

	rv := []*v2.Resource{}
	for _, pool := range agentPools.Items {
		agentTokens, err := o.client.AgentTokens.List(ctx, pool.ID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to list agentTokens: %w", err)
		}
		for _, agentToken := range agentTokens.Items {
			resource, err := newAgentTokenResource(agentToken, parentResourceID)
			if err != nil {
				return nil, "", nil, fmt.Errorf("baton-terraform-cloud: failed to create agentToken resource: %w", err)
			}

			rv = append(rv, resource)
		}
	}

	var nextPage string
	if agentPools.CurrentPage < agentPools.TotalPages {
		nextPage = strconv.Itoa(page + 1)
	}

	return rv, nextPage, nil, nil
}

// Entitlements always returns an empty slice for secrets.
func (o *agentTokenBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for secrets since they don't have any entitlements.
func (o *agentTokenBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newAgentTokenBuilder(client *client.Client) *agentTokenBuilder {
	return &agentTokenBuilder{
		client: client,
	}
}
