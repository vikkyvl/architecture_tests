package result

type Reviewer struct{}

func NewReviewer() *Reviewer {
	return new(Reviewer)
}
