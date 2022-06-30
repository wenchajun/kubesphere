/*
Copyright 2022 KubeSphere Authors

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

package v2

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"net/http"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

type Opensearch struct {
	Client *opensearch.Client
	index  string
}

func New(address string, basicAuth bool, username, password, index string) (*Opensearch, error) {
	var client *opensearch.Client
	var err error

	if !basicAuth {
		username = ""
		password = ""
	}

	client, err = opensearch.NewClient(opensearch.Config{
		Addresses: []string{address},
		Username:  username,
		Password:  password,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	})

	return &Opensearch{Client: client, index: index}, err
}

func (o *Opensearch) Search(indices string, body []byte, scroll bool) ([]byte, error) {
	opts := []func(request *opensearchapi.SearchRequest){
		o.Client.Search.WithContext(context.Background()),
		o.Client.Search.WithIndex(indices),
		o.Client.Search.WithTrackTotalHits(true),
		o.Client.Search.WithIgnoreUnavailable(true),
		o.Client.Search.WithBody(bytes.NewBuffer(body)),
	}
	if scroll {
		opts = append(opts, o.Client.Search.WithScroll(time.Minute))
	}

	response, err := o.Client.Search(opts...)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.IsError() {
		return nil, parseError(response)
	}

	return ioutil.ReadAll(response.Body)
}

func (o *Opensearch) Scroll(id string) ([]byte, error) {
	response, err := o.Client.Scroll(
		o.Client.Scroll.WithContext(context.Background()),
		o.Client.Scroll.WithScrollID(id),
		o.Client.Scroll.WithScroll(time.Minute))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.IsError() {
		return nil, parseError(response)
	}

	b, err := ioutil.ReadAll(response.Body)
	return b, err
}

func (o *Opensearch) ClearScroll(scrollId string) {
	response, _ := o.Client.ClearScroll(
		o.Client.ClearScroll.WithContext(context.Background()),
		o.Client.ClearScroll.WithScrollID(scrollId))
	defer response.Body.Close()
}

func (o *Opensearch) GetTotalHitCount(v interface{}) int64 {
	m, _ := v.(map[string]interface{})
	f, _ := m["value"].(float64)
	return int64(f)
}

func parseError(response *opensearchapi.Response) error {
	var e map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&e); err != nil {
		return err
	} else {
		// Print the response status and error information.
		e, _ := e["error"].(map[string]interface{})
		return fmt.Errorf("type: %v, reason: %v", e["type"], e["reason"])
	}
}