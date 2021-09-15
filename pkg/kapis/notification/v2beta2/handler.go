package v2beta2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/emicklei/go-restful"
	"kubesphere.io/api/notification/v2beta2"
	nm "kubesphere.io/kubesphere/pkg/simple/client/notification"
)

const (
	VerifyAPI = "/api/v2/verify"
)

type handler struct {
	option *nm.Options
}

type Result struct {
	Code    int    `json:"Status"`
	Message string `json:"Message"`
}
type notification struct {
	Config   v2beta2.Config   `json:"config"`
	Receiver v2beta2.Receiver `json:"receiver"`
}

func newHandler(option *nm.Options) *handler {
	return &handler{
		option,
	}
}

func (h handler) Verify(request *restful.Request, response *restful.Response) {
	opt := h.option
	if opt == nil || len(opt.Endpoint) == 0 {
		response.WriteAsJson(Result{
			http.StatusBadRequest,
			"The corresponding Notification Manager path could not be found",
		})
	}
	host := opt.Endpoint
	notification := notification{}
	reqBody, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	err = json.Unmarshal(reqBody, &notification)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	receiver := notification.Receiver
	user := request.PathParameter("user")

	if receiver.Labels["type"] == "tenant" {
		if user != receiver.Labels["user"] {
			response.WriteAsJson(Result{
				http.StatusForbidden,
				"No enough permissions",
			})
			return
		}
	}
	if receiver.Labels["type"] == "global" {
		if user != "" {
			response.WriteAsJson(Result{
				http.StatusForbidden,
				"No enough permissions",
			})
			return
		}
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s", host, VerifyAPI), bytes.NewReader(reqBody))
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}
	req.Header = request.Request.Header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// return 500
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	var result Result
	err = json.Unmarshal(body, &result)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	response.WriteAsJson(result)
}
