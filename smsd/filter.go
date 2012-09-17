package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

type Filter struct {
	path string
	cmd  *exec.Cmd
	in   *json.Decoder
	out  *json.Encoder
}

func NewFilter(path string) (*Filter, error) {
	if len(path) == 0 || path[0] != '|' {
		return nil, fmt.Errorf("Bad filter path: '%s'", path)
	}
	return &Filter{path: path[1:]}, nil
}

func (f *Filter) Path() string {
	return f.path
}

func (f *Filter) Filter(msg *Msg) (bool, error) {
	if f.cmd == nil {
		f.cmd = exec.Command(f.path)
		stdin, err := f.cmd.StdinPipe()
		if err != nil {
			return true, err
		}
		stdout, err := f.cmd.StdoutPipe()
		if err != nil {
			return true, err
		}
		f.cmd.Stderr = os.Stderr
		err = f.cmd.Start()
		if err != nil {
			f.cmd = nil
			return true, err
		}
		f.in = json.NewDecoder(stdout)
		f.out = json.NewEncoder(stdin)
	}

	err := f.out.Encode(msg)
	if err != nil {
		f.cmd.Wait()
		f.cmd = nil
		return true, err
	}
	var mm map[string]interface{}
	err = f.in.Decode(&mm)
	if err != nil {
		f.cmd.Wait()
		f.cmd = nil
		return true, err
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
	if v, ok := mm["Body"]; ok {
		if body, ok := v.(string); ok {
			msg.Body = body
		}
	}
	return true, nil
}
