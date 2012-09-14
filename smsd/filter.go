package main

import (
	"encoding/json"
	"fmt"
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
		return nil, fmt.Errorf("Bad filter: '%s'", path)
	}
	return &Filter{path: path}, nil
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
	var v map[string]interface{}
	err = f.in.Decode(&v)
	if err != nil {
		f.cmd.Wait()
		f.cmd = nil
		return true, err
	}
	if v == nil {
		return false, nil
	}
	if len(v) == 0 {
		return true, nil
	}

	return true, nil
}
