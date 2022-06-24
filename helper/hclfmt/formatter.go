package hclfmt

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// HCL2Formatter tracks all user inputted settings
// as we parse through file(s)
type HCL2Formatter struct {
	ShowDiff, Write, File bool
	Output                io.Writer
	parser                *hclparse.Parser
}

// NewHCL2Formatter creates a new formatter, ready to format configuration files.
func NewHCL2Formatter() *HCL2Formatter {
	return &HCL2Formatter{
		parser: hclparse.NewParser(),
	}
}

func (f *HCL2Formatter) Format(path string) (int, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	var bytesModified int

	if path == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "path is empty, cannot format",
			Detail:   "path is empty, cannot format",
		})
		return bytesModified, diags
	}

	if f.parser == nil {
		f.parser = hclparse.NewParser()
	}
	s, err := os.Stat(path)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "error finding file info",
			Detail:   fmt.Sprintf("%s", err),
		})
		return bytesModified, diags

	}

	//is there a better way to do this logic less ugly
	if s.IsDir() && f.File {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "cannot pass directory as a file",
			Detail:   "stop it now",
		})
		return bytesModified, diags
	}

	if !s.IsDir() && f.File {
		return f.formatFile(path, bytesModified)
	}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		diag := &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Cannot read hcl directory",
			Detail:   err.Error(),
		}
		diags = append(diags, diag)
		return bytesModified, diags
	}

	for _, fileInfo := range fileInfos {
		filename := filepath.Join(path, fileInfo.Name())
		if fileInfo.IsDir() {
			var tempBytesModified int
			f.Format(filename)
			bytesModified += tempBytesModified
		}
		continue
	}

	return bytesModified, diags
}

func (f *HCL2Formatter) processFile(filename string) ([]byte, error) {

	if f.Output == nil {
		f.Output = os.Stdout
	}

	var in io.Reader
	var err error

	if filename == "-" {
		in = os.Stdin
		f.ShowDiff = false
	} else {
		in, err = os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %s", filename, err)
		}
	}

	inSrc, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %s", filename, err)
	}

	_, diags := f.parser.ParseHCL(inSrc, filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL %s", filename)
	}

	outSrc := hclwrite.Format(inSrc)

	if bytes.Equal(inSrc, outSrc) {
		if filename == "-" {
			_, _ = f.Output.Write(outSrc)
		}

		return nil, nil
	}

	if filename != "-" {
		s := []byte(fmt.Sprintf("%s\n", filename))
		_, _ = f.Output.Write(s)
	}

	if f.Write {
		if filename == "-" {
			_, _ = f.Output.Write(outSrc)
		} else {
			if err := ioutil.WriteFile(filename, outSrc, 0644); err != nil {
				return nil, err
			}
		}
	}

	if f.ShowDiff {
		diff, err := bytesDiff(inSrc, outSrc, filename)
		if err != nil {
			return outSrc, fmt.Errorf("failed to generate diff for %s: %s", filename, err)
		}
		_, _ = f.Output.Write(diff)
	}

	return outSrc, nil
}

func (f *HCL2Formatter) formatFile(path string, bytesModified int) (int, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	data, err := f.processFile(path)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("encountered an error while formatting %s", path),
			Detail:   err.Error(),
		})
	}
	bytesModified += len(data)
	return bytesModified, diags
}

func bytesDiff(b1, b2 []byte, path string) (data []byte, err error) {
	f1, err := ioutil.TempFile("", "")
	if err != nil {
		return
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	f2, err := ioutil.TempFile("", "")
	if err != nil {
		return
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	_, _ = f1.Write(b1)
	_, _ = f2.Write(b2)

	data, err = exec.Command("diff", "--label=old/"+path, "--label=new/"+path, "-u", f1.Name(), f2.Name()).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	return
}