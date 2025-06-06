package connector

import (
	"context"
	"io"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-terraform-cloud/pkg/client"
)

type Connector struct {
	client *client.Client
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (d *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		newOrganizationBuilder(d.client),
		newUserBuilder(d.client),
		newProjectBuilder(d.client),
		newWorkspaceBuilder(d.client),
		newTeamBuilder(d.client),
		newAgentTokenBuilder(d.client),
	}
}

// Asset takes an input AssetRef and attempts to fetch it using the connector's authenticated http client
// It streams a response, always starting with a metadata object, following by chunked payloads for the asset.
func (d *Connector) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

// Metadata returns metadata about the connector.
func (d *Connector) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "My Baton Connector",
		Description: "The template implementation of a baton connector.",
		AccountCreationSchema: &v2.ConnectorAccountCreationSchema{
			FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
				"email": {
					DisplayName: "Email",
					Required:    true,
					Description: "The email address of the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Email",
					Order:       1,
				},
				"organizationName": {
					DisplayName: "Organization Name",
					Required:    true,
					Description: "The name of the organization to which the user will belong.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "organizationName",
					Order:       2,
				},
				"teamNames": {
					DisplayName: "Team Names",
					Required:    false,
					Description: "The names of the teams to which the user will belong. if not provided, the user will be added to the default \"owners\" team.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringListField{
						StringListField: &v2.ConnectorAccountCreationSchema_StringListField{},
					},
					Placeholder: "Team Names",
					Order:       3,
				},
			},
		},
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials
// to be sure that they are valid.
func (d *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	return nil, nil
}

// New returns a new instance of the connector.
func New(ctx context.Context, token, address string) (*Connector, error) {
	client, err := client.New(token, address)
	if err != nil {
		return nil, err
	}
	return &Connector{
		client: client,
	}, nil
}
