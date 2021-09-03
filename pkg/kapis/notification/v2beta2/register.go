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

package v2beta2

import (
	"net/http"

	"github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubesphere.io/kubesphere/pkg/api"
	"kubesphere.io/kubesphere/pkg/apiserver/runtime"
)

var GroupVersion = schema.GroupVersion{Group: "notification.kubesphere.io", Version: "v2beta2"}

func AddToContainer(container *restful.Container) error {
	h := newHandler()
	ws := runtime.NewWebService(GroupVersion)
	ws.Route(ws.POST("/verify").Reads("").To(h.Verify).Returns(http.StatusOK, api.StatusOK, http.Response{}.Body)).
		Doc("Provide validation for notification-manager information")
	container.Add(ws)
	return nil
}
