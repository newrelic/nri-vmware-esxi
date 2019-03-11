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

// newClient creates a govmomi.Client
func newClient(vmURL string, vmUsername string, vmPassword string, validateSSL bool) (*govmomi.Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Parse URL from string
	url, err := soap.ParseURL(vmURL)
	if err != nil {
		return nil, err
	}

	// Override username and/or password as required
	setCredentials(url, vmUsername, vmPassword)
	// Connect and log in to ESX or vCenter
	return govmomi.NewClient(ctx, url, validateSSL)
}

func close(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func logout(client *govmomi.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := client.Logout(ctx)
	if err != nil {
		log.Error(err.Error())
	}
}
