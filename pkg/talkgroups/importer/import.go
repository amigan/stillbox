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
	"time"

	"github.com/google/uuid"

	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/database"
	"dynatron.me/x/stillbox/pkg/talkgroups"
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
	sysn, has := talkgroups.StoreFrom(ctx).SystemName(ctx, sys)
	if !has {
		return nil, talkgroups.ErrNoSuchSystem
	}

	importedFrom := jsontypes.Metadata{
		"from": "RadioReference",
		"time": time.Now(),
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
			metadata := jsontypes.Metadata{
				"imported": importedFrom,
			}
			tgt := talkgroups.TG(sys, tgid)
			mode := fields[2]
			if strings.Contains(mode, "E") {
				metadata["encrypted"] = true
			}
			tags := []string{fields[5]}
			gn := groupName // must take a copy
			tgs = append(tgs, talkgroups.Talkgroup{
				Talkgroup: database.Talkgroup{
					ID:       uuid.New(),
					Tgid:     int32(tgt.Talkgroup),
					SystemID: int32(tgt.System),
					Name:     &fields[4],
					AlphaTag: &fields[3],
					TgGroup:  &gn,
					Metadata: metadata,
					Tags:     tags,
					Weight:   1.0,
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
