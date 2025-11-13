package forms_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"

	"dynatron.me/x/stillbox/pkg/talkgroups/xport"
)

func perr(err error) {
	if err != nil {
		panic(err)
	}
}

func makeExportRequest(ej *xport.ExportJob, url string) *http.Request {
	var buf bytes.Buffer
	body := multipart.NewWriter(&buf)

	perr(body.WriteField("type", string(ej.Type)))

	perr(body.WriteField("systemID", strconv.Itoa(int(ej.SystemID))))

	perr(body.WriteField("talkgroups", "[407:3,4]"))

	w, err := body.CreateFormFile("template", ej.TemplateFileName)
	perr(err)

	_, err = w.Write(ej.Template)
	perr(err)

	body.Close()

	r, err := http.NewRequest(http.MethodPost, url, &buf)
	perr(err)

	r.Header.Set("Content-Type", body.FormDataContentType())

	return r
}

func makeFunkyJSONExportRequest(ej *xport.ExportJob, url string) *http.Request {
	var buf bytes.Buffer
	je := json.NewEncoder(&buf)

	err := je.Encode(ej)
	perr(err)

	r, err := http.NewRequest(http.MethodPost, url, &buf)
	perr(err)
	r.Header.Set("Content-Type", "application/json")

	return r
}
