package v2beta2

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"kubesphere.io/api/notification/v2beta2"

	"github.com/emicklei/go-restful"
)

const (
	VerifyUrl = "http://notification-manager-svc.kubesphere-monitoring-system.svc:19093/api/v2/verify"
)

type handler struct{}

type Result struct {
	Code    int    `json:"Status"`
	Message string `json:"Message"`
}

func newHandler() handler {
	return handler{}
}

func (h handler) Verify(request *restful.Request, response *restful.Response) {

	config := v2beta2.Config{}
	reqBody, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	err = json.Unmarshal([]byte(reqBody), &config)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	user := request.PathParameter("user")
	if config.Labels["type"] == "tenant" {
		if user != config.Labels["user"] {
			response.WriteAsJson(Result{
				403,
				" No enough permissions",
			})
			return
		}
	}

	if user != config.Labels["user"] {
		if config.Labels["type"] != "default" && config.Labels["type"] != "global" {
			response.WriteAsJson(Result{
				403,
				" No enough permissions",
			})
			return
		}
	}

	req, err := http.NewRequest("POST", VerifyUrl, request.Request.Body)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

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
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		response.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}

	response.WriteAsJson(result)
}
