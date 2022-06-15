package checks

import (
	"fmt"
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
		Kind:    GetKind(c),
		Type:    c.Type,
		Address: "127.0.0.1:8080", // todo (YOU ARE HERE)
		Path:    c.Path,
		Method:  http.MethodGet,
	}
}

type Checker interface {
	Check(*Query) *QueryResult
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

func (c *checker) Check(q *Query) *QueryResult {
	switch q.Type {
	case "http":
		return c.checkHTTP(q)
	default:
		return c.checkTCP(q)
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

func (c *checker) checkHTTP(q *Query) *QueryResult {
	qr := &QueryResult{
		Kind:      q.Kind,
		Timestamp: c.now(),
		Result:    Pending,
	}

	u := q.Address + q.Path
	request, err := http.NewRequest(q.Method, u, nil)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		qr.Result = Failure
		return qr
	}

	result, err := c.httpClient.Do(request)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		qr.Result = Failure
		return qr
	}

	b, err := ioutil.ReadAll(result.Body)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		// let the status code dictate query result
	} else {
		qr.Output = string(b)
	}

	if result.StatusCode < 400 {
		qr.Result = Success
	} else {
		qr.Result = Failure
	}

	return qr
}
