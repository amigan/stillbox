package database

import (
	"strconv"
)

func (d GetTalkgroupsRow) GetTalkgroup() Talkgroup         { return d.Talkgroup }
func (d GetTalkgroupsRow) GetSystem() System               { return d.System }
func (d GetTalkgroupsRow) GetLearned() bool                { return d.Talkgroup.Learned }
func (g GetTalkgroupRow) GetTalkgroup() Talkgroup          { return g.Talkgroup }
func (g GetTalkgroupRow) GetSystem() System                { return g.System }
func (g GetTalkgroupRow) GetLearned() bool                 { return g.Talkgroup.Learned }
func (g GetTalkgroupsPRow) GetTalkgroup() Talkgroup        { return g.Talkgroup }
func (g GetTalkgroupsPRow) GetSystem() System              { return g.System }
func (g GetTalkgroupsPRow) GetLearned() bool               { return g.Talkgroup.Learned }
func (g GetTalkgroupsBySystemRow) GetTalkgroup() Talkgroup { return g.Talkgroup }
func (g GetTalkgroupsBySystemRow) GetSystem() System       { return g.System }
func (g GetTalkgroupsBySystemRow) GetLearned() bool        { return g.Talkgroup.Learned }

func (g GetTalkgroupsBySystemPRow) GetTalkgroup() Talkgroup { return g.Talkgroup }
func (g GetTalkgroupsBySystemPRow) GetSystem() System       { return g.System }
func (g GetTalkgroupsBySystemPRow) GetLearned() bool        { return g.Talkgroup.Learned }
func (g Talkgroup) GetTalkgroup() Talkgroup                 { return g }
func (g Talkgroup) GetSystem() System                       { return System{ID: int(g.SystemID)} }
func (g Talkgroup) GetLearned() bool                        { return false }

func (g Talkgroup) String() string {
	return g.StringTag(true)
}

func (g Talkgroup) StringTag(withTag bool) string {
	switch {
	case withTag && g.AlphaTag != nil:
		return *g.AlphaTag
	case g.Name != nil && g.TGGroup != nil:
		return *g.TGGroup + " " + *g.Name
	case g.Name != nil:
		return *g.Name + " [" + strconv.Itoa(int(g.TGID)) + "]"
	case g.TGGroup != nil:
		return *g.TGGroup + " [" + strconv.Itoa(int(g.TGID)) + "]"
	}

	return strconv.Itoa(int(g.TGID))
}
