package config

import "github.com/conductorone/baton-sdk/pkg/field"

var (
	TokenField = field.StringField(
		"token",
		field.WithRequired(true),
		field.WithIsSecret(true),
		field.WithDisplayName("API Token"),
		field.WithDescription("The API token used to authenticate with Terraform Cloud."),
	)
	AddressField = field.StringField(
		"address",
		field.WithRequired(false),
		field.WithDefaultValue("https://app.terraform.io"),
		field.WithDisplayName("Address"),
		field.WithDescription("The address of the Terraform instance. Default: https://app.terraform.io"),
	)
	ConfigurationFields = []field.SchemaField{TokenField, AddressField}
)

//go:generate go run ./gen

var Config = field.NewConfiguration(
	ConfigurationFields,
	field.WithConnectorDisplayName("Terraform Cloud"),
	field.WithHelpUrl("/docs/baton/terraform-cloud"),
	field.WithIconUrl("/static/app-icons/terraform-cloud.svg"),
)
