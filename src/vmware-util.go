package main

import (
	"context"
	"io"
	"net/url"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
)

func setCredentials(u *url.URL, un string, pw string) {
	// Override username if provided
	if un != "" {
		var password string
		var ok bool

		if u.User != nil {
			password, ok = u.User.Password()
		}

		if ok {
			u.User = url.UserPassword(un, password)
		} else {
			u.User = url.User(un)
		}
	}

	// Override password if provided
	if pw != "" {
		var username string

		if u.User != nil {
			username = u.User.Username()
		}

		u.User = url.UserPassword(username, pw)
	}
}

// newClient creates a govmomi.Client for use in the examples
func newClient(ctx context.Context) (*govmomi.Client, error) {
	// Parse URL from string
	u, err := soap.ParseURL(vmURL)
	if err != nil {
		return nil, err
	}

	// Override username and/or password as required
	setCredentials(u, vmUsername, vmPassword)

	// Connect and log in to ESX or vCenter
	return govmomi.NewClient(ctx, u, validateSSL)
}

func close(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func logout(ctx context.Context, client *govmomi.Client) {
	err := client.Logout(ctx)
	if err != nil {
		log.Error(err.Error())
	}
}
