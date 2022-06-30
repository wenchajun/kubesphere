/*
Copyright 2020 KubeSphere Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package os

import (
	"context"
	"fmt"
	"kubesphere.io/kubesphere/pkg/simple/client/os/query"

	"kubesphere.io/kubesphere/pkg/simple/client/os/versions"
	v2 "kubesphere.io/kubesphere/pkg/simple/client/os/versions/v2"
	"kubesphere.io/kubesphere/pkg/utils/esutil"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"


)

const (
	OpenSearchV2 = "2"
)

// OpenSearch client
type Client struct {
	host      string
	basicAuth bool
	username  string
	password  string
	version   string
	index     string

	c   versions.Client
	mux sync.Mutex
}

func NewClient(host string, basicAuth bool, username, password, indexPrefix, version string) (*Client, error) {
	var err error
	os := &Client{
		host:      host,
		basicAuth: basicAuth,
		username:  username,
		password:  password,
		version:   version,
		index:     indexPrefix,
	}

	switch os.version {
	case OpenSearchV2:
		os.c, err = v2.New(os.host, os.basicAuth, os.username, os.password, os.index)
	case "":
		os.c = nil
	default:
		return nil, fmt.Errorf("unsupported opensearch version %s", os.version)
	}

	return os, err
}

func (c *Client) loadClient() error {
	// Check if Elasticsearch client has been initialized.
	if c.c != nil {
		return nil
	}

	// Create Elasticsearch client.
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.c != nil {
		return nil
	}

	// Detect OpenSearch server version using Info API.
	// Info API is backward compatible across v5, v6 and v7.
	osv2, err := v2.New(c.host, c.basicAuth, c.username, c.password, c.index)
	if err != nil {
		return err
	}

	res, err := osv2.Client.Info(
		osv2.Client.Info.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	var b map[string]interface{}
	if err = jsoniter.NewDecoder(res.Body).Decode(&b); err != nil {
		return err
	}
	if res.IsError() {
		// Print the response status and error information.
		e, _ := b["error"].(map[string]interface{})
		return fmt.Errorf("[%s] type: %v, reason: %v", res.Status(), e["type"], e["reason"])
	}

	// get the major version
	version, _ := b["version"].(map[string]interface{})
	number, _ := version["number"].(string)
	if number == "" {
		return fmt.Errorf("failed to detect elastic version number")
	}

	var vc versions.Client
	v := strings.Split(number, ".")[0]
	switch v {
	case OpenSearchV2:
		vc, err = v2.New(c.host, c.basicAuth, c.username, c.password, c.index)
	default:
		err = fmt.Errorf("unsupported elasticsearch version %s", version)
	}

	if err != nil {
		return err
	}

	c.c = vc
	c.version = v
	return nil
}

func (c *Client) Search(builder *query.Builder, startTime, endTime time.Time, scroll bool) (*Response, error) {

	err := c.loadClient()
	if err != nil {
		return nil, err
	}

	// Initial Search
	body, err := builder.Bytes()
	if err != nil {
		return nil, err
	}

	res, err := c.c.Search(esutil.ResolveIndexNames(c.index, startTime, endTime), body, scroll)
	if err != nil {
		return nil, err
	}

	return parseResponse(res)
}

func (c *Client) Scroll(id string) (*Response, error) {

	err := c.loadClient()
	if err != nil {
		return nil, err
	}

	res, err := c.c.Scroll(id)
	if err != nil {
		return nil, err
	}

	return parseResponse(res)
}

func (c *Client) ClearScroll(id string) {
	if id != "" {
		c.c.ClearScroll(id)
	}
}

func (c *Client) GetTotalHitCount(v interface{}) int64 {
	return c.c.GetTotalHitCount(v)
}
