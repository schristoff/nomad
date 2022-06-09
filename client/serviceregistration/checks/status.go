package checks

type (
	AllocID string
	CheckID string
)

type Kind byte

const (
	Healthiness Kind = iota
	Readiness
)

type Result byte

const (
	Success Result = iota
	Critical
	Missing
)

func (r Result) String() string {
	switch r {
	case Success:
		return "success"
	case Critical:
		return "critical"
	default:
		return "missing"
	}
}

type QueryResult struct {
	ID     CheckID
	Kind   Kind
	Result Result
	Output string
	When   int64
}
