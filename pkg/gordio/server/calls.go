package server

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/google/uuid"
)

type callUploadRequest struct {
	Audio          []byte    `form:"audio"`
	AudioName      string    `form:"audioName"`
	AudioType      string    `form:"audioType"`
	DateTime       time.Time `form:"dateTime"`
	Frequencies    []int     `form:"frequencies"`
	Frequency      int       `form:"frequency"`
	Key            string    `form:"key"`
	Patches        []string  `form:"patches"`
	Source         int       `form:"source"`
	Sources        []string  `form:"sources"`
	System         int       `form:"system"`
	SystemLabel    string    `form:"systemLabel"`
	Talkgroup      int       `form:"talkgroup"`
	TalkgroupGroup string    `form:"talkgroupGroup"`
	TalkgroupLabel string    `form:"talkgroupLabel"`
	TalkgroupTag   string    `form:"talkgroupTag"`
}

func (s *Server) routeCallUpload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(1024 * 1024 * 2) // 2MB
	if err != nil {
		http.Error(w, "cannot parse form "+err.Error(), http.StatusBadRequest)
		return
	}

	keyUuid, err := uuid.Parse(r.Form.Get("key"))
	if err != nil {
		http.Error(w, "cannot parse key "+err.Error(), http.StatusBadRequest)
		return
	}
	apik, err := s.db.GetAPIKey(r.Context(), keyUuid)
	if err != nil {
		if database.IsNoRows(err) {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if apik.Disabled.Bool || (apik.Expires.Valid && time.Now().After(apik.Expires.Time)) {
		http.Error(w, "disabled", http.StatusUnauthorized)
		return
	}

	call := new(callUploadRequest)
	err = call.fill(r)
	if err != nil {
		http.Error(w, "cannot bind upload "+err.Error(), 500)
		return
	}

	w.Write([]byte(fmt.Sprintf("%#v", call)))
}

func (car *callUploadRequest) fill(r *http.Request) error {
	rv := reflect.ValueOf(car).Elem()
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		ff := rt.Field(i).Tag.Get("form")
		switch ff {
		case "audio":
			file, _, err := r.FormFile(ff)
			if err != nil {
				return fmt.Errorf("get form file: %w", err)
			}

			audioBytes, err := io.ReadAll(file)
			if err != nil {
				return fmt.Errorf("file read: %w", err)
			}

			f.SetBytes(audioBytes)
		case "dateTime":
			t, err := time.Parse(time.RFC3339, r.Form.Get(ff))
			if err != nil {
				return fmt.Errorf("parse time: %w", err)
			}
			f.Set(reflect.ValueOf(t))
		case "frequencies", "patches", "sources":
			val := strings.Trim(r.Form.Get(ff), "[]")
			if val == "" {
				continue
			}
			vals := strings.Split(val, ",")
			ar := make([]int, 0, len(vals))
			for _, v := range vals {
				i, err := strconv.Atoi(v)
				if err == nil {
					ar = append(ar, i)
				}
			}
			f.Set(reflect.ValueOf(ar))
		case "frequency", "talkgroup", "system", "source":
			val, err := strconv.Atoi(r.Form.Get(ff))
			if err != nil {
				return fmt.Errorf("atoi('%s'): %w", ff, err)
			}
			f.SetInt(int64(val))
		default:
			f.SetString(r.Form.Get(ff))
		}
	}

	return nil
}
