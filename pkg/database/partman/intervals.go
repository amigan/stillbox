package partman

import (
	"fmt"
	"time"
)

const (
	daysInWeek      = 7
	monthsInQuarter = 3
)

func getDailyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	upperBound = lowerBound.AddDate(0, 0, 1)

	return
}

func getWeeklyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -int(date.Weekday()-time.Monday))
	upperBound = lowerBound.AddDate(0, 0, daysInWeek)

	return
}

func getMonthlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
	upperBound = lowerBound.AddDate(0, 1, 0)

	return
}

func getQuarterlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	year, _, _ := date.Date()

	quarter := (int(date.Month()) - 1) / monthsInQuarter
	firstMonthOfTheQuarter := time.Month(quarter*monthsInQuarter + 1)

	lowerBound = time.Date(year, firstMonthOfTheQuarter, 1, 0, 0, 0, 0, time.UTC)
	upperBound = lowerBound.AddDate(0, monthsInQuarter, 0)

	return
}

func getYearlyBounds(date time.Time) (lowerBound, upperBound time.Time) {
	lowerBound = time.Date(date.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	upperBound = lowerBound.AddDate(1, 0, 0)

	return
}

func (p partition) Next(i int) partition {
	var t time.Time
	switch p.Interval {
	case Daily:
		t = p.Time.AddDate(0, 0, i)
	case Weekly:
		t = p.Time.AddDate(0, 0, i*daysInWeek)
	case Monthly:
		year, month, _ := p.Time.Date()

		t = time.Date(year, month+time.Month(i), 1, 0, 0, 0, 0, p.Time.Location())
	case Quarterly:
		t = p.Time.AddDate(0, i*monthsInQuarter, 0)
	case Yearly:
		year, _, _ := p.Time.Date()

		t = time.Date(year+i, 1, 1, 0, 0, 0, 0, p.Time.Location())
	}
	np := partition{
		ParentTable: p.ParentTable,
		Name:        p.Name,
		Schema:      p.Schema,
		Interval:    p.Interval,
		Time:        t,
	}

	np.setName()

	return np
}

func (p *partition) setName() {
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

func (p partition) Prev(i int) partition {
	var t time.Time
	switch p.Interval {
	case Daily:
		t = p.Time.AddDate(0, 0, -i)
	case Weekly:
		t = p.Time.AddDate(0, 0, -i*daysInWeek)
	case Monthly:
		year, month, _ := p.Time.Date()

		t = time.Date(year, month-time.Month(i), 1, 0, 0, 0, 0, p.Time.Location())
	case Quarterly:
		t = p.Time.AddDate(0, -i*monthsInQuarter, 0)
	case Yearly:
		year, _, _ := p.Time.Date()

		t = time.Date(year-i, 1, 1, 0, 0, 0, 0, p.Time.Location())
	}

	pp := partition{
		ParentTable: p.ParentTable,
		Name:        p.Name,
		Schema:      p.Schema,
		Interval:    p.Interval,
		Time:        t,
	}
	pp.setName()

	return pp

}
