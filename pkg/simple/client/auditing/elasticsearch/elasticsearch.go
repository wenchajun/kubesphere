/*
Copyright 2020 The KubeSphere Authors.

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

package elasticsearch

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strconv"

	"kubesphere.io/kubesphere/pkg/utils/stringutils"

	jsoniter "github.com/json-iterator/go"

	"kubesphere.io/kubesphere/pkg/simple/client/auditing"
	"kubesphere.io/kubesphere/pkg/simple/client/es"
	"kubesphere.io/kubesphere/pkg/simple/client/es/query"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

//From the parseToQueryPart function
type Source struct {
	Time                     string `json:"RequestReceivedTimestamp,omitempty"`
	Workspace                string `json:"Workspace,omitempty"`
	Verb                     string `json:"Verb,omitempty"`
	Level                    string `json:"Level,omitempty"`
	SourceIPs                string `json:"SourceIPs,omitempty"`
	AuditID                  string `json:"AuditID,omitempty"`
	Cluster                  string `json:"Cluster,omitempty"`
	Devops                   string `json:"Devops,omitempty"`
	Message                  string `json:"Message,omitempty"`
	RequestObject            string `json:"RequestObject,omitempty"`
	RequestReceivedTimestamp string `json:"RequestReceivedTimestamp,omitempty"`
	RequestURI               string `json:"RequestURI,omitempty"`
	Stage                    string `json:"Stage,omitempty"`
	StageTimestamp           string `json:"StageTimestamp,omitempty"`
	User                     `json:"User,omitempty"`
	ResponseStatus           `json:"ResponseStatus,omitempty"`
	ObjectRef                `json:"ObjectRef,omitempty"`
}

type ObjectRef struct {
	Namespace       string `json:"Namespace,omitempty"`
	Name            string `json:"Name,omitempty"`
	Resource        string `json:"Resource,omitempty"`
	Subresource     string `json:"Subresource,omitempty"`
	APIVersion      string `json:"APIVersion,omitempty"`
	APIGroup        string `json:"APIGroup,omitempty"`
	ResourceVersion string `json:"ResourceVersion,omitempty"`
	UID             string `json:"UID,omitempty"`
}

type User struct {
	Username string `json:"Username,omitempty"`
	Groups   string `json:"Groups,omitempty"`
	UID      string `json:"UID,omitempty"`
}

type ResponseStatus struct {
	Code     int         `json:"code,omitempty"`
	Metadata interface{} `json:"metadata,omitempty"`
}

type client struct {
	c *es.Client
}

func (c *client) SearchAuditingEvent(filter *auditing.Filter, from, size int64,
	sort string) (*auditing.Events, error) {

	b := query.NewBuilder().
		WithQuery(parseToQueryPart(filter)).
		WithSort("RequestReceivedTimestamp", sort).
		WithFrom(from).
		WithSize(size)

	resp, err := c.c.Search(b, filter.StartTime, filter.EndTime, false)
	if err != nil || resp == nil {
		return nil, err
	}

	events := &auditing.Events{Total: c.c.GetTotalHitCount(resp.Total)}
	for _, hit := range resp.AllHits {
		events.Records = append(events.Records, hit.Source)
	}
	return events, nil
}

func (c *client) CountOverTime(filter *auditing.Filter, interval string) (*auditing.Histogram, error) {

	if interval == "" {
		interval = "15m"
	}

	b := query.NewBuilder().
		WithQuery(parseToQueryPart(filter)).
		WithAggregations(query.NewAggregations().
			WithDateHistogramAggregation("RequestReceivedTimestamp", interval)).
		WithSize(0)

	resp, err := c.c.Search(b, filter.StartTime, filter.EndTime, false)
	if err != nil || resp == nil {
		return nil, err
	}

	h := auditing.Histogram{Total: c.c.GetTotalHitCount(resp.Total)}
	for _, bucket := range resp.Buckets {
		h.Buckets = append(h.Buckets,
			auditing.Bucket{Time: bucket.Key, Count: bucket.Count})
	}
	return &h, nil
}

func (c *client) StatisticsOnResources(filter *auditing.Filter) (*auditing.Statistics, error) {

	b := query.NewBuilder().
		WithQuery(parseToQueryPart(filter)).
		WithAggregations(query.NewAggregations().
			WithCardinalityAggregation("AuditID.keyword")).
		WithSize(0)

	resp, err := c.c.Search(b, filter.StartTime, filter.EndTime, false)
	if err != nil || resp == nil {
		return nil, err
	}

	return &auditing.Statistics{
		Resources: resp.Value,
		Events:    c.c.GetTotalHitCount(resp.Total),
	}, nil
}

func NewClient(options *auditing.Options) (auditing.Client, error) {
	c := &client{}

	var err error
	c.c, err = es.NewClient(options.Host, options.BasicAuth, options.Username, options.Password, options.IndexPrefix, options.Version)
	return c, err
}

func (c *client) ExportLogs(sf auditing.Filter, w io.Writer) error {

	var id string
	var data []string

	b := query.NewBuilder().
		WithQuery(parseToQueryPart(&sf)).
		WithSort("RequestReceivedTimestamp", "desc").
		WithFrom(0).
		WithSize(1000)

	resp, err := c.c.Search(b, sf.StartTime, sf.EndTime, true)
	if err != nil {
		return err
	}

	defer c.c.ClearScroll(id)

	id = resp.ScrollId
	for _, hit := range resp.AllHits {
		data = append(data, c.getSource(hit.Source).Verb)
		data = append(data, strconv.Itoa(c.getSource(hit.Source).ResponseStatus.Code))
		data = append(data, c.getSource(hit.Source).Workspace)
		data = append(data, c.getSource(hit.Source).Namespace)
		data = append(data, c.getSource(hit.Source).Resource)
		data = append(data, c.getSource(hit.Source).User.Username)
	}

	// limit to retrieve max 100 records
	for i := 0; i < 100; i++ {
		if i != 0 {
			data, id, err = c.scroll(id)
			if err != nil {
				return err
			}
		}
		if len(data) == 0 {
			return nil
		}

		count := 0
		output := new(bytes.Buffer)
		for _, l := range data {
			count++
			if count%6 != 0 {
				output.WriteString(fmt.Sprintf("%s\t", stringutils.StripAnsi(l)))
			} else {
				output.WriteString(fmt.Sprintf("%s\n", stringutils.StripAnsi(l)))
			}
		}
		_, err = io.Copy(w, output)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) scroll(id string) ([]string, string, error) {
	resp, err := c.c.Scroll(id)
	if err != nil {
		return nil, id, err
	}

	var data []string
	for _, hit := range resp.AllHits {
		data = append(data, c.getSource(hit.Source).Verb)
		data = append(data, strconv.Itoa(c.getSource(hit.Source).ResponseStatus.Code))
		data = append(data, c.getSource(hit.Source).Workspace)
		data = append(data, c.getSource(hit.Source).Namespace)
		data = append(data, c.getSource(hit.Source).Resource)
		data = append(data, c.getSource(hit.Source).User.Username)
		log.Println("---------")
	}
	return data, resp.ScrollId, nil
}

func (c *client) getSource(val interface{}) Source {

	s := Source{}

	bs, err := json.Marshal(val)
	if err != nil {
		return s
	}

	err = json.Unmarshal(bs, &s)
	if err != nil {
		return s
	}
	return s
}

func parseToQueryPart(f *auditing.Filter) *query.Query {
	if f == nil {
		return nil
	}

	var mini int32 = 1
	b := query.NewBool()

	bi := query.NewBool().WithMinimumShouldMatch(mini)
	for k, v := range f.ObjectRefNamespaceMap {
		bi.AppendShould(query.NewBool().
			AppendFilter(query.NewMatchPhrase("ObjectRef.Namespace.keyword", k)).
			AppendFilter(query.NewRange("RequestReceivedTimestamp").
				WithGTE(v)))
	}

	for k, v := range f.WorkspaceMap {
		bi.AppendShould(query.NewBool().
			AppendFilter(query.NewMatchPhrase("Workspace.keyword", k)).
			AppendFilter(query.NewRange("RequestReceivedTimestamp").
				WithGTE(v)))
	}

	b.AppendFilter(bi)

	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrase("ObjectRef.Namespace.keyword", f.ObjectRefNamespaces)).
		WithMinimumShouldMatch(mini))

	bi = query.NewBool().WithMinimumShouldMatch(mini)
	for _, ns := range f.ObjectRefNamespaceFuzzy {
		bi.AppendShould(query.NewWildcard("ObjectRef.Namespace.keyword", fmt.Sprintf("*"+ns+"*")))
	}
	b.AppendFilter(bi)

	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrase("Workspace.keyword", f.Workspaces)).
		WithMinimumShouldMatch(mini))

	bi = query.NewBool().WithMinimumShouldMatch(mini)
	for _, ws := range f.WorkspaceFuzzy {
		bi.AppendShould(query.NewWildcard("Workspace.keyword", fmt.Sprintf("*"+ws+"*")))
	}
	b.AppendFilter(bi)

	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrase("ObjectRef.Name.keyword", f.ObjectRefNames)).
		WithMinimumShouldMatch(mini))

	bi = query.NewBool().WithMinimumShouldMatch(mini)
	for _, name := range f.ObjectRefNameFuzzy {
		bi.AppendShould(query.NewWildcard("ObjectRef.Name.keyword", fmt.Sprintf("*"+name+"*")))
	}
	b.AppendFilter(bi)

	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrase("Verb.keyword", f.Verbs)).
		WithMinimumShouldMatch(mini))
	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrase("Level.keyword", f.Levels)).
		WithMinimumShouldMatch(mini))

	bi = query.NewBool().WithMinimumShouldMatch(mini)
	for _, ip := range f.SourceIpFuzzy {
		bi.AppendShould(query.NewWildcard("SourceIPs.keyword", fmt.Sprintf("*"+ip+"*")))
	}
	b.AppendFilter(bi)

	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrase("User.Username.keyword", f.Users)).
		WithMinimumShouldMatch(mini))

	bi = query.NewBool().WithMinimumShouldMatch(mini)
	for _, user := range f.UserFuzzy {
		bi.AppendShould(query.NewWildcard("User.Username.keyword", fmt.Sprintf("*"+user+"*")))
	}
	b.AppendFilter(bi)

	bi = query.NewBool().WithMinimumShouldMatch(mini)
	for _, group := range f.GroupFuzzy {
		bi.AppendShould(query.NewWildcard("User.Groups.keyword", fmt.Sprintf("*"+group+"*")))
	}
	b.AppendFilter(bi)

	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrasePrefix("ObjectRef.Resource", f.ObjectRefResources)).
		WithMinimumShouldMatch(mini))
	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrasePrefix("ObjectRef.Subresource", f.ObjectRefSubresources)).
		WithMinimumShouldMatch(mini))
	b.AppendFilter(query.NewBool().
		AppendShould(query.NewTerms("ResponseStatus.code", f.ResponseCodes)).
		WithMinimumShouldMatch(mini))
	b.AppendFilter(query.NewBool().
		AppendMultiShould(query.NewMultiMatchPhrase("ResponseStatus.status.keyword", f.ResponseStatus)).
		WithMinimumShouldMatch(mini))

	r := query.NewRange("RequestReceivedTimestamp")
	if !f.StartTime.IsZero() {
		r.WithGTE(f.StartTime)
	}
	if !f.EndTime.IsZero() {
		r.WithLTE(f.EndTime)
	}

	b.AppendFilter(r)

	return query.NewQuery().WithBool(b)
}
