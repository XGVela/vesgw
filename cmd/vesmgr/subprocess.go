/*
 *  Copyright (c) 2020 Mavenir.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
package main

import (
	"os"
	"os/exec"
)

type cmdRunner struct {
	exe  string
	args []string
	cmd  *exec.Cmd
}

func (r *cmdRunner) run(result chan error) {
	r.cmd = exec.Command(r.exe, r.args...)
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr
	err := r.cmd.Start()
	go func() {
		if err != nil {
			result <- err
		} else {
			result <- r.cmd.Wait()
		}
	}()
}

func (r *cmdRunner) kill() error {
	return r.cmd.Process.Kill()
}

func makeRunner(exe string, arg ...string) cmdRunner {
	r := cmdRunner{exe: exe, args: arg}
	return r
}
