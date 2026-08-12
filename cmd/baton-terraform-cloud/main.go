package main

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-sdk/pkg/cli"
	sdkConfig "github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/connectorrunner"
	"github.com/conductorone/baton-terraform-cloud/pkg/config"
	"github.com/conductorone/baton-terraform-cloud/pkg/connector"
)

const connectorName = "baton-terraform-cloud"

var version = "dev"

func main() {
	ctx := context.Background()

	sdkConfig.RunConnector(
		ctx,
		connectorName,
		version,
		config.Config,
		getConnector,
		connectorrunner.WithDefaultCapabilitiesConnectorBuilderV2(&connector.Connector{}),
		connectorrunner.WithSessionStoreEnabled(),
	)
}

func getConnector(ctx context.Context, cfg *config.TerraformCloud, _ *cli.ConnectorOpts) (connectorbuilder.ConnectorBuilderV2, []connectorbuilder.Opt, error) {
	cb, err := connector.New(ctx, cfg.Token, cfg.Address)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-terraform-cloud: error creating connector: %w", err)
	}

	return cb, nil, nil
}
