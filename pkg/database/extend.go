package database

import (
	"strconv"
)

func (d GetTalkgroupsRow) GetTalkgroup() Talkgroup                    { return d.Talkgroup }
func (d GetTalkgroupsRow) GetSystem() System                          { return d.System }
func (d GetTalkgroupsRow) GetLearned() bool                           { return d.Talkgroup.Learned }
func (g GetTalkgroupWithLearnedRow) GetTalkgroup() Talkgroup          { return g.Talkgroup }
func (g GetTalkgroupWithLearnedRow) GetSystem() System                { return g.System }
func (g GetTalkgroupWithLearnedRow) GetLearned() bool                 { return g.Talkgroup.Learned }
func (g GetTalkgroupsWithLearnedRow) GetTalkgroup() Talkgroup         { return g.Talkgroup }
func (g GetTalkgroupsWithLearnedRow) GetSystem() System               { return g.System }
func (g GetTalkgroupsWithLearnedRow) GetLearned() bool                { return g.Talkgroup.Learned }
func (g GetTalkgroupsWithLearnedPRow) GetTalkgroup() Talkgroup        { return g.Talkgroup }
func (g GetTalkgroupsWithLearnedPRow) GetSystem() System              { return g.System }
func (g GetTalkgroupsWithLearnedPRow) GetLearned() bool               { return g.Talkgroup.Learned }
func (g GetTalkgroupsWithLearnedBySystemRow) GetTalkgroup() Talkgroup { return g.Talkgroup }
func (g GetTalkgroupsWithLearnedBySystemRow) GetSystem() System       { return g.System }
func (g GetTalkgroupsWithLearnedBySystemRow) GetLearned() bool        { return g.Talkgroup.Learned }

func (g GetTalkgroupsWithLearnedBySystemPRow) GetTalkgroup() Talkgroup { return g.Talkgroup }
func (g GetTalkgroupsWithLearnedBySystemPRow) GetSystem() System       { return g.System }
func (g GetTalkgroupsWithLearnedBySystemPRow) GetLearned() bool        { return g.Talkgroup.Learned }
func (g Talkgroup) GetTalkgroup() Talkgroup                            { return g }
func (g Talkgroup) GetSystem() System                                  { return System{ID: int(g.SystemID)} }
func (g Talkgroup) GetLearned() bool                                   { return false }

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
