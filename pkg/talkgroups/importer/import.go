package importer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
)

type ImportSource string

const (
	ImportSrcRadioReference ImportSource = "radioreference"
)

var (
	ErrBadImportType = errors.New("unknown import type")
)

type importer interface {
	importTalkgroups(ctx context.Context, sys int, r io.Reader) ([]talkgroups.Talkgroup, error)
}

type ImportJob struct {
	Type     ImportSource `json:"type"`
	SystemID int          `json:"systemID"`
	Body     string       `json:"body"`

	importer `json:"-"`
}

func (ij *ImportJob) Import(ctx context.Context) ([]talkgroups.Talkgroup, error) {
	r := bytes.NewReader([]byte(ij.Body))

	switch ij.Type {
	case ImportSrcRadioReference:
		ij.importer = new(radioReferenceImporter)
	default:
		return nil, ErrBadImportType
	}

	return ij.importTalkgroups(ctx, ij.SystemID, r)
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

func (rr *radioReferenceImporter) importTalkgroups(ctx context.Context, sys int, r io.Reader) ([]talkgroups.Talkgroup, error) {
	sc := bufio.NewScanner(r)
	tgs := make([]talkgroups.Talkgroup, 0, 8)
	sysn, has := tgstore.From(ctx).SystemName(ctx, sys)
	if !has {
		return nil, tgstore.ErrNoSuchSystem
	}

	var groupName string
	state := rrsInitial
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

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
			if len(fields) != 6 {
				state = rrsGroupDesc
				groupName = ln
				continue
			}
			tgid, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}
			var metadata jsontypes.Metadata
			tgt := talkgroups.TG(sys, tgid)
			mode := fields[2]
			if strings.Contains(mode, "E") {
				metadata = make(jsontypes.Metadata)
				metadata["encrypted"] = true
			}
			tags := []string{fields[5]}
			gn := groupName // must take a copy
			tgs = append(tgs, talkgroups.Talkgroup{
				Talkgroup: database.Talkgroup{
					ID:       len(tgs), // need unique ID for the UI to track
					TGID:     int32(tgt.Talkgroup),
					SystemID: int32(tgt.System),
					Name:     &fields[4],
					AlphaTag: &fields[3],
					TGGroup:  &gn,
					Metadata: metadata,
					Tags:     tags,
					Weight:   1.0,
					Alert:    true,
				},
				System: database.System{
					ID:   sys,
					Name: sysn,
				},
			})

		}
	}

	if err := sc.Err(); err != nil {
		return tgs, err
	}

	return tgs, nil
}
