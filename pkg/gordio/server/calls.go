package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
)

type callUploadRequest struct {
	Audio          []byte    `form:"audio"`
	AudioName      string    `form:"audioName"`
	AudioType      time.Time `form:"audioType"`
	DateTime       string    `form:"dateTime"`
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
	callUpload := new(callUploadRequest)
	err := render.Bind(r, callUpload)
	if err != nil {
		http.Error(w, "cannot bind upload", 500)
		return
	}

	fmt.Printf("%#v\n", callUpload)
	w.Write([]byte("yay"))
}
