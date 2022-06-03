package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"
)

// SecureVariables is used to access the Nomad's secure variables store
type SecureVariables struct {
	client *Client
}

// SecureVariables returns a handle to the SecureVariables API endpoint
func (c *Client) SecureVariables() *SecureVariables {
	return &SecureVariables{client: c}
}

// SecureVariable structs are the cleartext secret material stored or to be
// stored in Nomad. Since they contains sensitive material, they should never
// be persisted to disk.
type SecureVariable struct {
	Namespace   string
	Path        string
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
	ModifyTime  time.Time
	Items       map[string]string
}

// SecureVariableMetadata is the metadata envelope for a Secure Variable, it
// is the list object and the shared data on a SecureVariable object.
type SecureVariableMetadata struct {
	Namespace   string
	Path        string
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
	ModifyTime  time.Time
}

// List lists all the secure variables
func (sv *SecureVariables) ListWithOptions(q *QueryOptions) ([]*SecureVariableMetadata, *QueryMeta, error) {
	var resp []*SecureVariableMetadata
	qm, err := sv.client.query("/v1/vars", &resp, q)
	if err != nil {
		return nil, nil, err
	}
	return resp, qm, nil
}

// PrefixList lists all the secure variables at a prefix in the current namespace
func (sv *SecureVariables) PrefixList(prefix string) ([]*SecureVariableMetadata, *QueryMeta, error) {
	q := &QueryOptions{
		Prefix: prefix,
	}
	return sv.ListWithOptions(q)
}

// List lists all the secure variables to which the caller has access.
func (sv *SecureVariables) List(opts ...SecureVariableQueryOption) ([]*SecureVariableMetadata, *QueryMeta, error) {
	q := &QueryOptions{}
	for _, o := range opts {
		o(q)
	}
	return sv.ListWithOptions(q)
}

type SecureVariableQueryOption func(*QueryOptions)

// WithAllNamespaces sets the Namespace in the QueryOptions to the wildcard string
func WithAllNamespaces() func(*QueryOptions) {
	return func(q *QueryOptions) {
		q.Namespace = "*"
	}
}

// WithNamespace sets the Namespace in the QueryOptions to the provided string
func WithNamespace(ns string) func(*QueryOptions) {
	return func(q *QueryOptions) {
		q.Namespace = ns
	}
}

// WithPrefix sets the Prefix in the QueryOptions to the provided string
func WithPrefix(p string) func(*QueryOptions) {
	return func(q *QueryOptions) {
		q.Prefix = p
	}
}

// WithOptions enables you to pass a pointer to an preconstructed QueryOptions
func WithQueryOptions(qo *QueryOptions) func(*QueryOptions) {
	return func(iq *QueryOptions) {
		*iq = *qo
	}
}

func buildQueryOptionFromOpts(opts ...SecureVariableQueryOption) *QueryOptions {
	q := new(QueryOptions)
	for _, o := range opts {
		o(q)
	}
	return q
}

// Read fetches a secure variable from Nomad including the Items map.
func (sv *SecureVariables) Read(path string, opts ...SecureVariableQueryOption) (*SecureVariable, *QueryMeta, error) {
	q := buildQueryOptionFromOpts(opts...)
	var out SecureVariable
	qm, err := sv.client.query(fmt.Sprintf("/v1/var/%s", path), &out, q)
	if err != nil {
		return nil, qm, err
	}
	return &out, qm, err
}

// Upsert upserts a secure variable into Nomad. This function will always over-
// write an existing value or make a new one if there is no error. For
// conditional application, use Create, Update, or UpsertWithCheckIndex.
func (sv *SecureVariables) Upsert(v *SecureVariable, w *WriteOptions) (*WriteMeta, error) {
	wm, err := sv.client.write(fmt.Sprintf("/v1/var/%s", v.Path), v, nil, w)
	return wm, err
}

// UpsertWithCheckIndex upserts a secure variable into Nomad if the ModifyIndex
// of the current object matches the ModifyIndex on any existing variable at the
// same path.
func (sv *SecureVariables) UpsertWithCheckIndex(
	v *SecureVariable, w *WriteOptions) (*SecureVariable, *WriteMeta, error) {
	return sv.upsertWithCheckIndexImpl(v, v.ModifyIndex, w)
}

// Create will add a new variable to the secure variable store. Fails if the
// variable already exists.
func (sv *SecureVariables) Create(
	v *SecureVariable, w *WriteOptions) (*SecureVariable, *WriteMeta, error) {
	return sv.upsertWithCheckIndexImpl(v, 0, w)
}

// Update updates an existing secure variable. Fails if the variable does not
// exist.
func (sv *SecureVariables) Update(
	v *SecureVariable, w *WriteOptions) (*SecureVariable, *WriteMeta, error) {
	return sv.upsertWithCheckIndexImpl(v, v.ModifyIndex, w)
}

func (sv *SecureVariables) upsertWithCheckIndexImpl(
	v *SecureVariable, ci uint64, w *WriteOptions) (*SecureVariable, *WriteMeta, error) {
	var out SecureVariable
	wm, err := sv.client.write(
		fmt.Sprintf("/v1/var/%s?cas=%v", v.Path, v.ModifyIndex),
		v, &out, w)
	return &out, wm, err
}

// Purge permanently deletes the Nomad secure variable at the given path.
func (sv *SecureVariables) PurgeVariableAtPath(path string, w *WriteOptions) (*WriteMeta, error) {
	wm, err := sv.client.delete(fmt.Sprintf("/v1/var/%v",
		path), nil, w)
	return wm, err
}

// PurgeWithCheckIndex permanently deletes a secure variable from Nomad if and
// only if the variable's ModifyIndex matches the provided checkIndex
func (sv *SecureVariables) PurgeWithCheckIndex(
	path string, checkIndex uint64, w *WriteOptions) (*WriteMeta, error) {
	wm, err := sv.client.delete(fmt.Sprintf("/v1/var/%v?cas=%v",
		path, checkIndex), nil, w)
	return wm, err
}

func (v *SecureVariable) Copy() *SecureVariable {
	out := *v
	out.Items = make(map[string]string, len(v.Items))
	for k, v := range v.Items {
		out.Items[k] = v
	}
	return &out
}

// Metadata returns a pointer to a copy of the secure variable's unencrypted
// values.
func (v *SecureVariable) Metadata() *SecureVariableMetadata {
	return &SecureVariableMetadata{
		Namespace:   v.Namespace,
		Path:        v.Path,
		CreateTime:  v.CreateTime,
		CreateIndex: v.CreateIndex,
		ModifyIndex: v.ModifyIndex,
		ModifyTime:  v.ModifyTime,
	}
}

func (sv SecureVariable) AsJSON() (string, error) {
	b, err := json.Marshal(sv)
	return string(b), err
}

func (sv SecureVariable) AsHCL() (string, error) {
	return sv.Render(hclTemplate)
}

func (sv SecureVariable) Render(tmpl string) (string, error) {
	var ob bytes.Buffer
	t, err := template.New("hclOut").Parse(strings.TrimSpace(hclTemplate))
	if err != nil {
		return "", err
	}

	err = t.Execute(&ob, sv)
	if err != nil {
		return "", err
	}
	return ob.String(), nil
}

var (
	hclTemplate = `
CreateIndex = {{.CreateIndex}}
CreateTime  = "{{.CreateTime}}"
ModifyIndex = {{.ModifyIndex}}
ModifyTime  = "{{.ModifyTime}}"
Namespace   = "{{.Namespace}}"
Path        = "{{.Path}}"

Items = {
{{range $k, $v := .Items}}{{printf "  %v = %q\n" $k $v}}{{end -}}
}
`
)
