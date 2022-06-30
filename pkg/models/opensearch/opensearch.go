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

package opensearch

import (
	"io"
	"kubesphere.io/kubesphere/pkg/api/oslog/v1alpha2"
	"kubesphere.io/kubesphere/pkg/simple/client/oslog"
)

type OpensearchOperator interface {
	GetCurrentStats(sf oslog.SearchFilter) (v1alpha2.APIResponse, error)
	CountLogsByInterval(sf oslog.SearchFilter, interval string) (v1alpha2.APIResponse, error)
	ExportLogs(sf oslog.SearchFilter, w io.Writer) error
	SearchLogs(sf oslog.SearchFilter, from, size int64, order string) (v1alpha2.APIResponse, error)
}

type opensearchOperator struct {
	c oslog.Client
}

func NewOpensearchOperator(client oslog.Client) OpensearchOperator {
	return &opensearchOperator{client}
}

func (l opensearchOperator) GetCurrentStats(sf oslog.SearchFilter) (v1alpha2.APIResponse, error) {
	res, err := l.c.GetCurrentStats(sf)
	return v1alpha2.APIResponse{Statistics: &res}, err
}

func (l opensearchOperator) CountLogsByInterval(sf oslog.SearchFilter, interval string) (v1alpha2.APIResponse, error) {
	res, err := l.c.CountLogsByInterval(sf, interval)
	return v1alpha2.APIResponse{Histogram: &res}, err
}

func (l opensearchOperator) ExportLogs(sf oslog.SearchFilter, w io.Writer) error {
	return l.c.ExportLogs(sf, w)
}

func (l opensearchOperator) SearchLogs(sf oslog.SearchFilter, from, size int64, order string) (v1alpha2.APIResponse, error) {
	res, err := l.c.SearchLogs(sf, from, size, order)
	return v1alpha2.APIResponse{Logs: &res}, err
}
