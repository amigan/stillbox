package server

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"
)

//const timeFormat = "2006-01-02T15:04:05.999Z07:00"

type callUploadRequest struct {
	Audio          []byte    `form:"audio"`
	AudioName      string    `form:"audioName"`
	AudioType      string    `form:"audioType"`
	DateTime       time.Time `form:"dateTime"`
	Frequencies    []int     `form:"frequencies"`
	Frequency      int       `form:"frequency"`
	Key            string    `form:"key"`
	Patches        []string  `form:"patches"`
	Source         string    `form:"source"`
	Sources        []string  `form:"sources"`
	System         string    `form:"system"`
	SystemLabel    string    `form:"systemLabel"`
	Talkgroup      int       `form:"talkgroup"`
	TalkgroupGroup string    `form:"talkgroupGroup"`
	TalkgroupLabel string    `form:"talkgroupLabel"`
	TalkgroupTag   string    `form:"talkgroupTag"`
}

func (car *callUploadRequest) Bind(r *http.Request) error {
	return nil
}

func (s *Server) routeCallUpload(w http.ResponseWriter, r *http.Request) {
	call := new(callUploadRequest)
	err := fillFields(r, call)
	if err != nil {
		http.Error(w, "cannot bind upload "+r.Header.Get("Content-Type")+err.Error(), 500)
		return
	}

	fmt.Printf("%#v\n", call)
	w.Write([]byte("yay"))
}

func fillFields(r *http.Request, v *callUploadRequest) error {
	rv := reflect.ValueOf(v).Elem()
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		ff := rt.Field(i).Tag.Get("form")
		switch ff {
		case "audio":
			file, _, err := r.FormFile(ff)
			if err != nil {
				return err
			}

			audioBytes, err := io.ReadAll(file)
			if err != nil {
				return err
			}

			f.SetBytes(audioBytes)
		case "dateTime":
			t, err := time.Parse(time.RFC3339, r.Form.Get(ff))
			if err != nil {
				return err
			}
			f.Set(reflect.ValueOf(t))
		case "frequencies":
		case "frequency", "talkgroup":
		case "patches", "sources":
		default:
			f.SetString(r.Form.Get(ff))
		}
	}

	return nil
}
