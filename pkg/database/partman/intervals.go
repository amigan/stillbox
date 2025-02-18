package partman

import (
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/common"
)

func (p Partition) Next(i int) Partition {
	var t time.Time
	switch p.Interval {
	case Daily:
		t = p.Time.AddDate(0, 0, i)
	case Weekly:
		t = p.Time.AddDate(0, 0, i*common.DaysInWeek)
	case Monthly:
		year, month, _ := p.Time.Date()

		t = time.Date(year, month+time.Month(i), 1, 0, 0, 0, 0, p.Time.Location())
	case Quarterly:
		t = p.Time.AddDate(0, i*common.MonthsInQuarter, 0)
	case Yearly:
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
	case Daily:
		suffix = t.Format("2006_01_02")
	case Weekly:
		year, week := t.ISOWeek()
		suffix = fmt.Sprintf("%d_w%02d", year, week)
	case Monthly:
		suffix = t.Format("2006_01")
	case Quarterly:
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
	case Yearly:
		suffix = t.Format("2006")
	default:
		panic(ErrInvalidInterval(p.Interval))
	}

	p.Name = fmt.Sprintf("%s_p_%s", p.ParentTable, suffix)
}

func (p Partition) Prev(i int) Partition {
	var t time.Time
	switch p.Interval {
	case Daily:
		t = p.Time.AddDate(0, 0, -i)
	case Weekly:
		t = p.Time.AddDate(0, 0, -i*common.DaysInWeek)
	case Monthly:
		year, month, _ := p.Time.Date()

		t = time.Date(year, month-time.Month(i), 1, 0, 0, 0, 0, p.Time.Location())
	case Quarterly:
		t = p.Time.AddDate(0, -i*common.MonthsInQuarter, 0)
	case Yearly:
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
	b := common.NewTimeBounder()
	switch p.Interval {
	case Daily:
		return b.GetDailyBounds(p.Time)
	case Weekly:
		return b.GetWeeklyBounds(p.Time)
	case Monthly:
		return b.GetMonthlyBounds(p.Time)
	case Quarterly:
		return b.GetQuarterlyBounds(p.Time)
	case Yearly:
		return b.GetYearlyBounds(p.Time)
	}

	panic("unknown interval!")
}
