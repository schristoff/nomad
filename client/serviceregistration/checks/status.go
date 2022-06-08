package checks

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

type QueryResult struct {
	Kind   Kind
	Result Result
	Output string
	When   int64
}
