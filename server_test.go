/*
 * DudelDu
 *
 * Copyright 2016 Matthias Ladkau. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the MIT
 * License, If a copy of the MIT License was not distributed with this
 * file, You can obtain one at https://opensource.org/licenses/MIT.
 */

package dudeldu

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"testing"
)

var testport = "localhost:9090"

func TestServer(t *testing.T) {

	DebugOutput = true

	var out bytes.Buffer

	// Collect the print output
	Print = func(v ...interface{}) {
		out.WriteString(fmt.Sprint(v...))
		out.WriteString("\n")
	}
	defer func() {
		Print = log.Print
	}()

	dds := NewServer(func(c net.Conn, err net.Error) {
		if err != nil {
			t.Error(err)
			return
		}

		c.Write([]byte("Hello"))

		c.Close()
	})

	var wg sync.WaitGroup
	wg.Add(1)

	err := dds.Run(":abc", &wg)
	if err == nil {
		t.Error("Unexpected error return:", err)
		return
	}

	wg.Add(1)

	go func() {
		err := dds.Run(testport, &wg)
		if err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Wait()

	// Server is now running

	ret, err := readSocket()

	if err != nil {
		t.Error(err)
		return
	}

	if ret != "Hello" {
		t.Error("Unexpected server response:", ret)
		return
	}

	wg.Add(1)

	dds.Shutdown()

	wg.Wait()
}

func readSocket() (string, error) {
	conn, err := net.Dial("tcp", testport)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	var buf bytes.Buffer
	io.Copy(&buf, conn)

	return buf.String(), nil
}
