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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessRunning(t *testing.T) {
	r := makeRunner("echo", "a")
	ch := make(chan error)
	r.run(ch)
	err := <-ch
	assert.Nil(t, err)
}

func TestProcessKill(t *testing.T) {
	r := makeRunner("sleep", "20")
	ch := make(chan error)
	r.run(ch)
	assert.Nil(t, r.kill())
	<-ch // wait and seee that kills is actually done
}

func TestProcessRunningFails(t *testing.T) {
	r := makeRunner("foobarbaz")
	ch := make(chan error)
	r.run(ch)
	err := <-ch
	assert.NotNil(t, err)
}
