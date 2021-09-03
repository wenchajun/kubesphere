package v2beta2

import (
	"encoding/json"

	"io"
	"net/http"

	"github.com/emicklei/go-restful"
)

type handler struct{}

type Result struct {
	Code    int    `json:"Status"`
	Message string `json:"Message"`
}

func newHandler() handler {
	return handler{}
}

const VerifyUrl = "http://notification-manager-svc.kubesphere-monitoring-system.svc:19093/api/v2/verify"

func (h handler) Verify(request *restful.Request, response *restful.Response) {

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
