package server

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/gordio/database"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type callUploadRequest struct {
	Audio          []byte `form:"audio"`
	AudioName      string
	AudioType      string    `form:"audioType"`
	DateTime       time.Time `form:"dateTime"`
	Frequencies    []int     `form:"frequencies"`
	Frequency      int       `form:"frequency"`
	Key            string    `form:"key"`
	Patches        []int     `form:"patches"`
	Source         int       `form:"source"`
	Sources        []int     `form:"sources"`
	System         int       `form:"system"`
	SystemLabel    string    `form:"systemLabel"`
	Talkgroup      int       `form:"talkgroup"`
	TalkgroupGroup string    `form:"talkgroupGroup"`
	TalkgroupLabel string    `form:"talkgroupLabel"`
	TalkgroupTag   string    `form:"talkgroupTag"`
}

func (car *callUploadRequest) ToAddCallParams(submitter int) database.AddCallParams {
	return database.AddCallParams{
		Submitter:   common.PtrTo(int32(submitter)),
		System:      car.System,
		Talkgroup:   car.Talkgroup,
		CallDate:    car.DateTime,
		AudioName:   common.PtrOrNull(car.AudioName),
		AudioBlob:   car.Audio,
		AudioType:   common.PtrOrNull(car.AudioType),
		Frequency:   car.Frequency,
		Frequencies: car.Frequencies,
		Patches:     car.Patches,
		TgLabel:     common.PtrOrNull(car.TalkgroupLabel),
		TgTag:       common.PtrOrNull(car.TalkgroupTag),
		TgGroup:     common.PtrOrNull(car.TalkgroupGroup),
		Source:      car.Source,
	}
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
	db := database.FromCtx(r.Context())
	apik, err := db.GetAPIKey(r.Context(), keyUuid)
	if err != nil {
		if database.IsNoRows(err) {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}

		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if (apik.Disabled != nil && *apik.Disabled) || (apik.Expires.Valid && time.Now().After(apik.Expires.Time)) {
		http.Error(w, "disabled", http.StatusUnauthorized)
		log.Error().Str("key", apik.ApiKey.String()).Msg("key disabled")
		return
	}

	if strings.Trim(r.Form.Get("test"), "\r\n") == "1" {
		// fudge the official response
		http.Error(w, "incomplete call data: no talkgroup", http.StatusExpectationFailed)
		return
	}

	call := new(callUploadRequest)
	err = call.fill(r)
	if err != nil {
		http.Error(w, "cannot bind upload "+err.Error(), http.StatusExpectationFailed)
		return
	}

	dbCall, err := db.AddCall(r.Context(), call.ToAddCallParams(apik.Owner))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		log.Error().Err(err).Msg("add call")
		return
	}

	w.Write([]byte("Call imported successfully."))

	_ = dbCall
}

func (car *callUploadRequest) fill(r *http.Request) error {
	rv := reflect.ValueOf(car).Elem()
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		fi := f.Interface()
		formField, has := rt.Field(i).Tag.Lookup("form")
		if !has {
			continue
		}
		switch v := fi.(type) {
		case []byte:
			file, hdr, err := r.FormFile(formField)
			if err != nil {
				return fmt.Errorf("get form file: %w", err)
			}

			car.AudioName = hdr.Filename
			audioBytes, err := io.ReadAll(file)
			if err != nil {
				return fmt.Errorf("file read: %w", err)
			}

			f.SetBytes(audioBytes)
		case time.Time:
			tval := r.Form.Get(formField)
			if iv, err := strconv.Atoi(tval); err == nil {
				f.Set(reflect.ValueOf(time.Unix(int64(iv), 0)))
				break
			}
			t, err := time.Parse(time.RFC3339, tval)
			if err != nil {
				return fmt.Errorf("parse time: %w", err)
			}
			f.Set(reflect.ValueOf(t))
		case []int:
			val := strings.Trim(r.Form.Get(formField), "[]")
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
		case int:
			val, err := strconv.Atoi(r.Form.Get(formField))
			if err != nil {
				return fmt.Errorf("atoi('%s'): %w", formField, err)
			}
			f.SetInt(int64(val))
		case string:
			f.SetString(r.Form.Get(formField))
		default:
			panic(fmt.Errorf("unsupported type %T", v))
		}
	}

	return nil
}
