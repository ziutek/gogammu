package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
)

type Filter struct {
	path   string
	cmd    *exec.Cmd
	in     *json.Decoder
	out    *json.Encoder
	stderr io.Reader
}

func NewFilter(path string) (*Filter, error) {
	if len(path) == 0 || path[0] != '|' {
		return nil, fmt.Errorf("bad filter path: '%s'", path)
	}
	return &Filter{path: path[1:]}, nil
}

func (f *Filter) Path() string {
	return f.path
}

// If error == nil bool means: accept/deny
func (f *Filter) Filter(msg *Msg) (bool, error) {
	needRetry := true
retry:
	if f.cmd == nil {
		f.cmd = exec.Command(f.path)
		stdin, err := f.cmd.StdinPipe()
		if err != nil {
			return false, err
		}
		stdout, err := f.cmd.StdoutPipe()
		if err != nil {
			return false, err
		}
		f.stderr, err = f.cmd.StderrPipe()
		if err != nil {
			return false, err
		}
		err = f.cmd.Start()
		if err != nil {
			f.cmd = nil
			return false, err
		}
		f.in = json.NewDecoder(stdout)
		f.out = json.NewEncoder(stdin)
	}

	err := f.out.Encode(msg)
	if err != nil {
		f.cmd.Wait()
		f.cmd = nil
		if needRetry {
			// If we cant send to filter process it probably exited last time.
			needRetry = false
			goto retry
		}
		return false, err
	}
	var mm map[string]interface{}
	err = f.in.Decode(&mm)
	if err != nil {
		if err == io.EOF {
			// Filter process has closed Stdout. Check for log in Stderr
			l, _ := ioutil.ReadAll(f.stderr)
			if len(l) > 0 {
				err = fmt.Errorf("%s stderr: %s", f.path, l)
			}
		}
		f.cmd.Wait()
		f.cmd = nil
		return false, err
	}
	if mm == nil {
		return false, nil
	}
	if len(mm) == 0 {
		// No modifications
		return true, nil
	}
	if v, ok := mm["Number"]; ok {
		if number, ok := v.(string); ok {
			msg.Number = number
		}
	}
	if v, ok := mm["SrcId"]; ok {
		if srcId, ok := v.(float64); ok && srcId > 0 {
			msg.SrcId = uint(srcId)
		}
	}
	if v, ok := mm["Rd"]; ok {
		if body, ok := v.(bool); ok {
			msg.Rd = body
		}
	}

	if v, ok := mm["Body"]; ok {
		if body, ok := v.(string); ok {
			msg.Body = body
		}
	}
	return true, nil
}
