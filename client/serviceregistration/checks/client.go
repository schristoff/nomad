package checks

import (
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"oss.indeed.com/go/libtime"
)

// A Query is derived from a structs.ServiceCheck and contains the minimal
// amount of information needed to actually execute that check.
type Query struct {
	Kind    Kind
	Type    string
	Address string
	Path    string // http only
	Method  string // http only
}

// GetKind determines whether the check is readiness or healthiness.
func GetKind(c *structs.ServiceCheck) Kind {
	if c != nil && c.OnUpdate == "ignore" {
		return Readiness
	}
	return Healthiness
}

// GetQuery extracts the needed info from c to actually execute the check.
func GetQuery(c *structs.ServiceCheck) *Query {
	return &Query{
		Kind: GetKind(c),
		Type: c.Type,
		// address
	}
}

type Checker interface {
	Check(*Query) (*QueryResult, error)
}

func New(log hclog.Logger) Checker {
	httpClient := cleanhttp.DefaultPooledClient()
	httpClient.Timeout = 1 * time.Minute
	return &checker{
		log:        log.Named("checks"),
		httpClient: httpClient,
		clock:      libtime.SystemClock(),
	}
}

type checker struct {
	log        hclog.Logger
	clock      libtime.Clock
	httpClient *http.Client
}

func (c *checker) now() int64 {
	return c.clock.Now().UTC().Unix()
}

func (c *checker) Check(q *Query) (*QueryResult, error) {
	switch q.Type {
	case "http":
		return c.checkHTTP(q)
	default:
		return c.checkTCP(q), nil
	}
}

func (c *checker) checkTCP(q *Query) *QueryResult {
	status := &QueryResult{
		Kind:      q.Kind,
		Timestamp: c.now(),
		Result:    Success,
	}
	if _, err := net.Dial("tcp", q.Address); err != nil {
		c.log.Info("check is failing", "kind", q.Kind, "address", q.Address, "error", err)
		status.Output = err.Error()
		status.Result = Failure
	}
	c.log.Trace("check is success", "kind", q.Kind, "address", q.Address)
	return status
}

func (c *checker) checkHTTP(q *Query) (*QueryResult, error) {
	status := &QueryResult{
		Kind:      q.Kind,
		Timestamp: c.now(),
		Result:    Pending,
	}

	u := q.Address + q.Path
	request, reqErr := http.NewRequest(q.Method, u, nil)
	if reqErr != nil {
		return status, reqErr
	}

	result, doErr := c.httpClient.Do(request)
	if doErr != nil {
		return status, doErr
	}

	b, bodyErr := ioutil.ReadAll(result.Body)
	if bodyErr != nil {
		return status, bodyErr
	}

	status.Output = string(b)
	if result.StatusCode < 400 {
		status.Result = Success
	} else {
		status.Result = Failure
	}

	return status, nil
}
