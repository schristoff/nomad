package hclfmt

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHCL2Formatter_Format(t *testing.T) {
	tt := []struct {
		Name           string
		Path           string
		FormatExpected bool
	}{
		{Name: "Unformatted file", Path: "testdata/unformatted.nomad", FormatExpected: true},
		{Name: "Formatted file", Path: "testdata/formatted.nomad"},
		{Name: "Directory", Path: "testdata/", FormatExpected: true},
	}

	for _, tc := range tt {
		tc := tc
		var buf bytes.Buffer
		f := NewHCL2Formatter()
		f.Output = &buf
		_, err := f.Format(tc.Path)
		if err != nil {
			t.Fatalf("the call to Format failed unexpectedly %s", err)
		}
		if buf.String() != "" && tc.FormatExpected == false {
			t.Errorf("format(%q) should contain the name of the formatted file(s), but got %q", tc.Path, buf.String())
		}
	}
}

func TestHCL2Formatter_Format_Write(t *testing.T) {

	var buf bytes.Buffer
	f := NewHCL2Formatter()
	f.Output = &buf
	f.Write = true

	unformattedData, err := ioutil.ReadFile("testdata/unformatted.nomad")
	if err != nil {
		t.Fatalf("failed to open the unformatted fixture %s", err)
	}

	tf, err := ioutil.TempFile("", "*.nomad")
	if err != nil {
		t.Fatalf("failed to create tempfile for test %s", err)
	}
	defer os.Remove(tf.Name())

	_, _ = tf.Write(unformattedData)
	tf.Close()

	_, diags := f.Format(tf.Name())
	if diags.HasErrors() {
		t.Fatalf("the call to Format() failed unexpectedly %s", err)
	}

	//lets re-read the tempfile which should now be formatted
	data, err := ioutil.ReadFile(tf.Name())
	if err != nil {
		t.Fatalf("failed to open the newly formatted fixture %s", err)
	}

	formattedData, err := ioutil.ReadFile("testdata/formatted.nomad")
	if err != nil {
		t.Fatalf("failed to open the formatted fixture %s", err)
	}

	if diff := cmp.Diff(string(data), string(formattedData)); diff != "" {
		t.Errorf("unexpected format output %s", diff)
	}
}

func TestHCL2Formatter_Format_ShowDiff(t *testing.T) {

	if _, err := exec.LookPath("diff"); err != nil {
		t.Skip("skipping test because diff is not in the executable PATH")
	}

	var buf bytes.Buffer
	f := HCL2Formatter{
		Output:   &buf,
		ShowDiff: true,
	}

	_, err := f.Format("testdata/unformatted.nomad")
	if err != nil {
		t.Fatalf("the call to Format failed unexpectedly %s", err)
	}

	diffHeader := `
--- old/testdata/unformatted.nomad
+++ new/testdata/unformatted.nomad
@@ -9,8 +9,8 @@`

	if !strings.Contains(buf.String(), diffHeader) {
		t.Errorf("expected buf to contain a file diff, but instead we got %s", buf.String())
	}

}
