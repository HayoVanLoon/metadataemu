package metadataemu

import (
	"cloud.google.com/go/compute/metadata"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client interface {
	Get(path string) (string, error)
	ProjectID() (string, error)
}

type client struct {
	port   string
	apiKey string
	real   bool
}

func NewClient(port, apiKey string, live bool) Client {
	if live {
		return &metadata.Client{}
	}
	return &client{port: port, apiKey: apiKey}
}

func (c *client) Get(path string) (string, error) {
	if c.apiKey == "" {
		return "", nil
	}
	url := fmt.Sprintf("http://localhost:%s%s&apiKey=%s", c.port, path, c.apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not query metadata emulator: %s", err)
	}
	bs, err := ioutil.ReadAll(resp.Body)
	return string(bs), err
}

func (c *client) ProjectID() (string, error) {
	return c.Get(projectIdUrl)
}
