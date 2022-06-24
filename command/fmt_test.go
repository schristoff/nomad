package command

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

const testdata = "helper/hclfmt/testdata/"

func TestFmt(t *testing.T) {
	s := &strings.Builder{}

	ui := cli.NewMockUi()
	cmd := &FormatCommand{Meta: Meta{Ui: ui}}
	// filepath := filepath.Join(testdata, "formatted.nomad")
	args := []string{"-check=true", "-path=../helper/hclfmt/testdata/formatted.nomad"}

	if code := cmd.Run(args); code != 0 {
		err := ui.ErrorWriter.String()
		t.Fatalf(
			"Wanted non-zero exit , got %d \n\nStderr:\n\n%s",
			code,
			err)
	}
	expected := ""
	assert.Equal(t, expected, strings.TrimSpace(s.String()))
}

func TestFmt_unfomattedTemlateDirectory(t *testing.T) {
	ui := cli.NewMockUi()
	cmd := &FormatCommand{Meta: Meta{Ui: ui}}

	args := []string{"-check=true", fmt.Sprintf("-path=%s", filepath.Join(testdata))}

	if code := cmd.Run(args); code != 3 {
		t.Fatalf("receieved non-three error code, %d", code)
	}
}

const (
	unformattedHCL = `
	ami_filter_name ="amzn2-ami-hvm-*-x86_64-gp2"
ami_filter_owners =[ "137112412989" ]
`
	formattedHCL = `
	ami_filter_name   = "amzn2-ami-hvm-*-x86_64-gp2"
ami_filter_owners = ["137112412989"]
`
)

func TestFmt_Directory(t *testing.T) {

	tests := []struct {
		name                  string
		formatArgs            []string // arguments passed to format
		alreadyPresentContent map[string]string
		fileCheck
	}{
		{
			name:       "nested formats only main dir",
			formatArgs: []string{"-path="},
			alreadyPresentContent: map[string]string{
				"foo/bar/baz.pkr.hcl":         unformattedHCL,
				"foo/bar/baz/woo.pkrvars.hcl": unformattedHCL,
				"potato":                      unformattedHCL,
				"foo/bar/potato":              unformattedHCL,
				"bar.pkr.hcl":                 unformattedHCL,
				"-":                           unformattedHCL,
			},
			fileCheck: fileCheck{
				expectedContent: map[string]string{
					"foo/bar/baz.pkr.hcl":         formattedHCL,
					"foo/bar/baz/woo.pkrvars.hcl": unformattedHCL,
					"potato":                      unformattedHCL,
					"foo/bar/potato":              unformattedHCL,
					"bar.pkr.hcl":                 formattedHCL,
					"-":                           unformattedHCL,
				}},
		},
		{
			name:       "nested",
			formatArgs: []string{},
			alreadyPresentContent: map[string]string{
				"foo/bar/baz.pkr.hcl":         unformattedHCL,
				"foo/bar/baz/woo.pkrvars.hcl": unformattedHCL,
				"bar.pkr.hcl":                 unformattedHCL,
				"-":                           unformattedHCL,
			},
			fileCheck: fileCheck{
				expectedContent: map[string]string{
					"foo/bar/baz.pkr.hcl":         unformattedHCL,
					"foo/bar/baz/woo.pkrvars.hcl": unformattedHCL,
					"bar.pkr.hcl":                 formattedHCL,
					"-":                           unformattedHCL,
				}},
		},
	}

	ui := cli.NewMockUi()
	cmd := &FormatCommand{Meta: Meta{Ui: ui}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDirectory, err := ioutil.TempDir("", "test-dir-*")
			if err != nil {
				t.Fatalf("unable to create tempDirectory, %v", err)
			}
			defer os.RemoveAll(tempDirectory)

			createFiles(tempDirectory, tt.alreadyPresentContent)

			if code := cmd.Run([]string{fmt.Sprintf("-path=%s", tempDirectory)}); code != 0 {
				out := ui.OutputWriter.String()
				err := ui.ErrorWriter.String()
				t.Fatalf(
					"Wanted non-zero exit , got %d for test case: %s.\n\nStdout:\n\n%s\n\nStderr:\n\n%s",
					code,
					tt.name,
					out,
					err)
			}

			tt.fileCheck.verify(t, tempDirectory)
		})
	}
}

func TestFmt_File(t *testing.T) {

	tests := []struct {
		name                  string
		formatFile            string // arguments passed to format
		alreadyPresentContent map[string]string
		fileCheck
		isErr bool
	}{
		{
			name:       "format file",
			formatFile: "potato.nomad",
			alreadyPresentContent: map[string]string{
				"foo/bar/baz.nomad":    unformattedHCL,
				"foo/bar/baz/woo.hcl":  unformattedHCL,
				"potato.nomad":         unformattedHCL,
				"foo/bar/potato.nomad": unformattedHCL,
				"bar.hcl":              unformattedHCL,
				"-":                    unformattedHCL,
			},
			fileCheck: fileCheck{
				expectedContent: map[string]string{
					"foo/bar/baz.nomad":    unformattedHCL,
					"foo/bar/baz/woo.hcl":  unformattedHCL,
					"potato.nomad":         formattedHCL,
					"foo/bar/potato.nomad": unformattedHCL,
					"bar.hcl":              unformattedHCL,
					"-":                    unformattedHCL,
				}},
			isErr: false,
		},
		{
			name:       "directory thinks it is a file",
			formatFile: "foo",
			alreadyPresentContent: map[string]string{
				"foo/bar/baz.nomad":    unformattedHCL,
				"foo/bar/baz/woo.hcl":  unformattedHCL,
				"potato.nomad":         unformattedHCL,
				"foo/bar/potato.nomad": unformattedHCL,
				"bar.hcl":              unformattedHCL,
				"-":                    unformattedHCL,
			},
			fileCheck: fileCheck{
				expectedContent: map[string]string{
					"foo/bar/baz.nomad":    unformattedHCL,
					"foo/bar/baz/woo.hcl":  unformattedHCL,
					"potato.nomad":         unformattedHCL,
					"foo/bar/potato.nomad": unformattedHCL,
					"bar.hcl":              unformattedHCL,
					"-":                    unformattedHCL,
				}},
			isErr: true,
		},
	}

	ui := cli.NewMockUi()
	cmd := &FormatCommand{Meta: Meta{Ui: ui}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDirectory, err := ioutil.TempDir("", "test-dir-*")
			if err != nil {
				t.Fatalf("unable to create tempDirectory, %v", err)
			}
			defer os.RemoveAll(tempDirectory)

			createFiles(tempDirectory, tt.alreadyPresentContent)
			fmtCmd := fmt.Sprintf("-file=%s", tempDirectory+"/"+tt.formatFile)
			code := cmd.Run([]string{fmtCmd})
			out := ui.OutputWriter.String()

			tt.fileCheck.verify(t, tempDirectory)

			//there's a better way to do this logic
			if tt.isErr {
				assert.Equal(t, 1, code)
				return
			}

			if code != 0 {
				err := ui.ErrorWriter.String()
				t.Fatalf(
					"Wanted non-zero exit , got %d for test case: %s.\n\nStdout:\n\n%s\n\nStderr:\n\n%s",
					code,
					tt.name,
					out,
					err)
			}

		})
	}
}

// func Test_fmt_pipe(t *testing.T) {

// 	tc := []struct {
// 		piped    string
// 		command  []string
// 		env      []string
// 		expected string
// 	}{
// 		{unformattedHCL, []string{"fmt", "-"}, nil, formattedHCL},
// 		{formattedHCL, []string{"fmt", "-"}, nil, formattedHCL},
// 	}

// 	for _, tc := range tc {
// 		t.Run(fmt.Sprintf("echo %q | packer %s", tc.piped, tc.command), func(t *testing.T) {
// 			Stdin = strings.NewReader(tc.piped)
// 			bs, err := p.Output()
// 			if err != nil {
// 				t.Fatalf("Error occurred running command %v: %s", err, bs)
// 			}
// 			if diff := cmp.Diff(tc.expected, string(bs)); diff != "" {
// 				t.Fatalf("Error in diff: %s", diff)
// 			}
// 		})
// 	}
// }

func createFiles(dir string, content map[string]string) {
	for relPath, content := range content {
		contentPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(contentPath), 0777); err != nil {
			panic(err)
		}
		if err := ioutil.WriteFile(contentPath, []byte(content), 0666); err != nil {
			panic(err)
		}
		log.Printf("created tmp file: %s", contentPath)
	}
}

type fileCheck struct {
	expected, notExpected []string
	expectedContent       map[string]string
}

func (fc fileCheck) verify(t *testing.T, dir string) {
	for _, f := range fc.expectedFiles() {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("Expected to find %s: %v", f, err)
		}
	}
	for _, f := range fc.notExpected {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("Expected to not find %s", f)
		}
	}
	for file, expectedContent := range fc.expectedContent {
		content, err := ioutil.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("ioutil.ReadFile: %v", err)
		}
		if diff := cmp.Diff(expectedContent, string(content)); diff != "" {
			t.Errorf("content of %s differs: %s", file, diff)
		}
	}
}

func (fc fileCheck) expectedFiles() []string {
	expected := fc.expected
	for file := range fc.expectedContent {
		expected = append(expected, file)
	}
	return expected
}
