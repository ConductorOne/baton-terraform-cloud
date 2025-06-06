package client

import (
	"github.com/hashicorp/go-tfe"
)

var (
	PageSize = 100
)

type Client struct {
	// https://app.terraform.io
	// https://developer.hashicorp.com/terraform/cloud-docs/api-docs
	*tfe.Client
}

func New(token, address string) (*Client, error) {
	config := &tfe.Config{
		// defaults to https://app.terraform.io
		Address:           address,
		Token:             token,
		RetryServerErrors: true,
	}

	client, err := tfe.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client: client,
	}, nil
}

func ListOptions(pageNumber int) tfe.ListOptions {
	return tfe.ListOptions{
		PageNumber: pageNumber,
		PageSize:   PageSize,
	}
}
