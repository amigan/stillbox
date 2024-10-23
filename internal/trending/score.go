package trending

type Score[K comparable] struct {
	ID          K
	Score       float64
	Probability float64
	Expectation float64
	Maximum     float64
	KLScore     float64
	Count       float64
	RecentCount float64
}

type Scores[K comparable] []Score[K]

func (s Scores[K]) Len() int {
	return len(s)
}

func (s Scores[K]) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Scores[K]) Less(i, j int) bool {
	return s[i].Score > s[j].Score
}

func (s Scores[K]) take(count int) Scores[K] {
	if count >= len(s) {
		return s
	}
	return s[0 : count-1]
}

func (s Scores[K]) threshold(t float64) Scores[K] {
	for i := range s {
		if s[i].Score < t {
			return s[0:i]
		}
	}
	return s
}
