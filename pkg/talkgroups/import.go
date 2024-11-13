package talkgroups

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"dynatron.me/x/stillbox/pkg/database"
)

type ImportSource string

const (
	ImportSrcRadioReference ImportSource = "radioreference"
)

var (
	ErrBadImportType = errors.New("unknown import type")
)

type importer interface {
	importTalkgroups(sys int, r io.Reader) ([]Talkgroup, error)
}

type ImportJob struct {
	Type ImportSource `json:"type"`
	SystemID int `json:"systemID"`
	Body string `json:"body"`

	importer `json:"-"`
}

func (ij *ImportJob) Import() ([]Talkgroup, error) {
	r := bytes.NewReader([]byte(ij.Body))

	switch ij.Type {
	case ImportSrcRadioReference:
		ij.importer = &radioReferenceImporter{}
	default:
		return nil, ErrBadImportType
	}
	return ij.importTalkgroups(ij.SystemID, r)
}

type radioReferenceImporter struct {
}

type rrState int
const (
	rrsInitial rrState = iota
	rrsGroupDesc
	rrsTG
)

var rrRE = regexp.MustCompile(`DEC\s+HEX\s+Mode\s+Alpha Tag\s+Description\s+Tag`)

func (rr *radioReferenceImporter) importTalkgroups(sys int, r io.Reader) ([]Talkgroup, error) {
	sc := bufio.NewScanner(r)
	tgs := make([]Talkgroup, 0, 8)

	var groupName string
	state := rrsInitial
	for sc.Scan() {
		ln := strings.Trim(sc.Text(), " \t\r\n")

		switch state {
		case rrsInitial:
			groupName = ln
			state++
		case rrsGroupDesc:
			if rrRE.MatchString(ln) {
				state++
			}
		case rrsTG:
			fields := strings.Split(ln, "\t")
			if len(fields) < 6 {
				state = rrsGroupDesc
				groupName = ln
				continue
			}
			tgid, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}
			var metadata []byte
			tgt := TG(sys, tgid)
			mode := fields[2]
			if strings.Contains(mode, "E") {
				metadata, _ = json.Marshal(&struct{
					Encrypted bool `json:"encrypted"`
				}{true})
			}
			tags := []string{fields[5]}
			tgs = append(tgs, Talkgroup{
				Talkgroup: database.Talkgroup{
					ID: tgt.Pack(),
					Tgid: int32(tgt.Talkgroup),
					SystemID: int32(tgt.System),
					Name: &fields[4],
					AlphaTag: &fields[3],
					TgGroup: &groupName,
					Metadata: metadata,
					Tags: tags,
					Weight: 1.0,
				},
				System: database.System{
					ID: sys,
					Name: "<imported>",
				},
			})

		}
	}

	if err := sc.Err(); err != nil {
		return tgs, err
	}

	return tgs, nil
}
