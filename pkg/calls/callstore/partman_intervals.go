package callstore

import (
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/common"
)

func (p Partition) Next(i int) Partition {
	var t time.Time
	switch p.Interval {
	case common.Daily:
		t = p.Time.AddDate(0, 0, i)
	case common.Weekly:
		t = p.Time.AddDate(0, 0, i*common.DaysInWeek)
	case common.Monthly:
		year, month, _ := p.Time.Date()

		t = time.Date(year, month+time.Month(i), 1, 0, 0, 0, 0, p.Time.Location())
	case common.Quarterly:
		t = p.Time.AddDate(0, i*common.MonthsInQuarter, 0)
	case common.Yearly:
		year, _, _ := p.Time.Date()

		t = time.Date(year+i, 1, 1, 0, 0, 0, 0, p.Time.Location())
	}
	np := Partition{
		ParentTable: p.ParentTable,
		Name:        p.Name,
		Schema:      p.Schema,
		Interval:    p.Interval,
		Time:        t,
	}

	np.setName()

	return np
}

func (p *Partition) setName() {
	t := p.Time
	var suffix string

	switch p.Interval {
	case common.Daily:
		suffix = t.Format("2006_01_02")
	case common.Weekly:
		year, week := t.ISOWeek()
		suffix = fmt.Sprintf("%d_w%02d", year, week)
	case common.Monthly:
		suffix = t.Format("2006_01")
	case common.Quarterly:
		year, month, _ := t.Date()

		var quarter int

		switch {
		case month >= 1 && month <= 3:
			quarter = 1
		case month >= 4 && month <= 6:
			quarter = 2
		case month >= 7 && month <= 9:
			quarter = 3
		case month >= 10 && month <= 12:
			quarter = 4
		}

		suffix = fmt.Sprintf("%d_q%d", year, quarter)
	case common.Yearly:
		suffix = t.Format("2006")
	default:
		panic(common.ErrInvalidInterval(p.Interval))
	}

	p.Name = fmt.Sprintf("%s_p_%s", p.ParentTable, suffix)
}

func (p Partition) Prev(i int) Partition {
	var t time.Time
	switch p.Interval {
	case common.Daily:
		t = p.Time.AddDate(0, 0, -i)
	case common.Weekly:
		t = p.Time.AddDate(0, 0, -i*common.DaysInWeek)
	case common.Monthly:
		year, month, _ := p.Time.Date()

		t = time.Date(year, month-time.Month(i), 1, 0, 0, 0, 0, p.Time.Location())
	case common.Quarterly:
		t = p.Time.AddDate(0, -i*common.MonthsInQuarter, 0)
	case common.Yearly:
		year, _, _ := p.Time.Date()

		t = time.Date(year-i, 1, 1, 0, 0, 0, 0, p.Time.Location())
	}

	pp := Partition{
		ParentTable: p.ParentTable,
		Name:        p.Name,
		Schema:      p.Schema,
		Interval:    p.Interval,
		Time:        t,
	}
	pp.setName()

	return pp

}

func (p Partition) Range() (time.Time, time.Time) {
	return common.NewTimeBounder(common.WithDefaultBounds(p.Interval)).Bounds(p.Time)
}
