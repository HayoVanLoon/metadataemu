// Copyright 2022 Hayo van Loon. All rights reserved.
// Use of this source code is governed by a licence
// that can be found in the LICENSE file.

package metadataemu

import (
	"cloud.google.com/go/compute/metadata"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type Client interface {
	Get(path string) (string, error)
	ProjectID() (string, error)
}

type client struct {
	scheme string
	apiKey string
	real   bool
}

// NewClient creates a new metadata client.
// If scheme is left empty, it will try to use environment variable
// `GCE_METADATA_HOST`.
// If live is set to `true`, port and apiKey will be ignored and the 'real'
// Google metadata client is returned.
func NewClient(scheme, apiKey string, live bool) Client {
	if live {
		return metadata.NewClient(nil)
	}
	if scheme == "" {
		// Use same environment parameter as real client
		scheme = fmt.Sprintf("http://%s", os.Getenv("GCE_METADATA_HOST"))
	}
	return &client{scheme: scheme, apiKey: apiKey}
}

func (c *client) Get(path string) (string, error) {
	url := fmt.Sprintf("%s%s%s", c.scheme, ComputeMetadataPrefix, path)
	if c.apiKey != "" {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		url = fmt.Sprintf("%s%sapiKey=%s", url, sep, c.apiKey)
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not query metadata emulator: %s", err)
	}
	bs, err := ioutil.ReadAll(resp.Body)
	return string(bs), err
}

func (c *client) ProjectID() (string, error) {
	return c.Get(EndPointProjectId)
}
