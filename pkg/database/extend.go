package database

func (d GetTalkgroupsRow) GetTalkgroup() Talkgroup { return d.Talkgroup }
func (d GetTalkgroupsRow) GetSystem() System       { return d.System }
func (d GetTalkgroupsRow) GetLearned() bool        { return d.Learned }
func (g GetTalkgroupsWithLearnedRow) GetTalkgroup() Talkgroup            { return g.Talkgroup }
func (g GetTalkgroupsWithLearnedRow) GetSystem() System                  { return g.System }
func (g GetTalkgroupsWithLearnedRow) GetLearned() bool                   { return g.Learned }
func (g GetTalkgroupsWithLearnedBySystemRow) GetTalkgroup() Talkgroup    { return g.Talkgroup }
func (g GetTalkgroupsWithLearnedBySystemRow) GetSystem() System          { return g.System }
func (g GetTalkgroupsWithLearnedBySystemRow) GetLearned() bool           { return g.Learned }
func (g Talkgroup) GetTalkgroup() Talkgroup                              { return g }
func (g Talkgroup) GetSystem() System                                    { return System{ID: int(g.SystemID)} }
func (g Talkgroup) GetLearned() bool                                     { return false }
