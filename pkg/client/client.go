package client

import (
	"log"

	"github.com/hashicorp/go-tfe"
)

var (
	PageSize = 100
)

type Client struct {
	*tfe.Client
}

func New(token, address string) *Client {
	if address == "" {
		address = "https://app.terraform.io"
	}

	config := &tfe.Config{
		Address:           address,
		Token:             token,
		RetryServerErrors: true,
	}

	client, err := tfe.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}
	return &Client{
		Client: client,
	}
}

func ListOptions(pageNumber int) tfe.ListOptions {
	return tfe.ListOptions{
		PageNumber: pageNumber,
		PageSize:   PageSize,
	}
}
