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
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"testing"

	"devt.de/common/testutil"
)

const testRequest = `
GET /mylist HTTP/1.1
Host: localhost:9091
User-Agent: VLC/2.2.1 LibVLC/2.2.1
Range: bytes=0-
Connection: close
Icy-MetaData: 1` +
	"\r\n\r\n"

const testRequest2 = `
GET /mylist2 HTTP/1.1
Host: localhost:9091
User-Agent: VLC/2.2.1 LibVLC/2.2.1
Range: bytes=656-
Connection: close
Icy-MetaData: 1` +
	"\r\n\r\n"

const testRequest3 = `
GET /bach/cello_suite1 HTTP/1.1
Host: localhost:9091
User-Agent: Mozilla/5.0 (Windows NT 6.3; WOW64; rv:48.0) Gecko/20100101 Firefox/99.0
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8
Accept-Language: en-US,en;q=0.5
Accept-Encoding: gzip, deflate
Authorization: Basic d2ViOndlYg==
Connection: keep-alive
Upgrade-Insecure-Requests: 1
Cache-Control: max-age=0
`

const testRequest4 = "GET /bach/cello_suite1 HTTP/1.1\r\nHost: localhost:9091\r\n" +
	"User-Agent: Mozilla/5.0 (Windows NT 6.3; WOW64; rv:48.0) Gecko/20100101 Firefox/48.0\r\n" +
	"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\n" +
	"Accept-Encoding: gzip, deflate\r\n" +
	"Authorization: Basic d2ViOndlYg==\r\n" +
	"Connection: keep-alive\r\nUpgrade-Insecure-Requests: 1\r\nCache-Control: max-age=0"

const testRequest5 = `
GET /mylist2 HTTP/1.1
Host: localhost:9091
User-Agent: VLC/2.2.1 LibVLC/2.2.1
Range: bytes=656-
Authorization: Basic erghb4
Connection: close
Icy-MetaData: 1` +
	"\r\n\r\n"

/*
testNetError is am error for testing
*/
type testNetError struct {
}

func (t *testNetError) Error() string {
	return "TestNetError"
}

func (t *testNetError) Timeout() bool {
	return false
}

func (t *testNetError) Temporary() bool {
	return false
}

type testPlaylistFactory struct {
	RetPlaylist Playlist
}

func (tp *testPlaylistFactory) Playlist(path string, shuffle bool) Playlist {
	if path == "/testpath" {
		return tp.RetPlaylist
	}
	return nil
}

var testTitle = "Test Title"

/*
testPlaylist is a playlist for testing
*/
type testPlaylist struct {
	Frames [][]byte
	Errors []error
	fp     int
}

func (tp *testPlaylist) Name() string {
	return "TestPlaylist"
}

func (tp *testPlaylist) ContentType() string {
	return "Test/Content"
}

func (tp *testPlaylist) Artist() string {
	return "Test Artist"
}

func (tp *testPlaylist) Title() string {
	return testTitle
}

func (tp *testPlaylist) Frame() ([]byte, error) {
	var err error
	f := tp.Frames[tp.fp]
	if tp.Errors != nil {
		err = tp.Errors[tp.fp]
	}
	tp.fp++
	return f, err
}

func (tp *testPlaylist) ReleaseFrame([]byte) {
}

func (tp *testPlaylist) Finished() bool {
	return tp.fp == len(tp.Frames)
}

func (tp *testPlaylist) Close() {
	tp.fp = 0
}

func TestRequestServing(t *testing.T) {

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

	drh := NewDefaultRequestHandler(&testPlaylistFactory{}, false, false, "")
	testConn := &testutil.ErrorTestingConnection{}

	// Test a path not found

	drh.defaultServeRequest(testConn, "tester", false, 0, "")

	if testConn.Out.String() != "HTTP/1.1 404 Not found\r\n\r\n" {
		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

	// Test straight forward case - serving a stream without meta data

	drh = NewDefaultRequestHandler(&testPlaylistFactory{&testPlaylist{
		[][]byte{[]byte("12"), nil, []byte("3")},
		[]error{nil, nil, errors.New("TestError")},
		0}}, false, false, "")
	testConn = &testutil.ErrorTestingConnection{}

	out.Reset()

	drh.defaultServeRequest(testConn, "/testpath", false, 0, "")

	if testConn.Out.String() != "ICY 200 OK\r\n"+
		"Content-Type: Test/Content\r\n"+
		"icy-name: TestPlaylist\r\n"+
		"\r\n"+
		"123" {

		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

	if out.String() != "Serve request path:/testpath Metadata support:false Offset:0\n"+
		"Written bytes: 0\n"+
		"Sending: Test Title - Test Artist\n"+
		"Empty frame for: Test Title - Test Artist (Error: <nil>)\n"+
		"Error while retrieving playlist data: TestError\n"+
		"Serve request path:/testpath complete\n" {
		t.Error("Unexpected out string:", out.String())
		return
	}

	// Test case when sending meta data

	oldMetaDataInterval := MetaDataInterval
	MetaDataInterval = 5
	defer func() {
		MetaDataInterval = oldMetaDataInterval
	}()

	tpl := &testPlaylist{[][]byte{[]byte("123"), []byte("4567"), []byte("0123"), []byte("456789")}, nil, 0}
	drh = NewDefaultRequestHandler(&testPlaylistFactory{tpl}, false, false, "")
	testConn = &testutil.ErrorTestingConnection{}

	drh.defaultServeRequest(testConn, "/testpath", true, 0, "")

	// Meta data is 3*16=48 bytes - text is 39 bytes, padding is 9 bytes

	if testConn.Out.String() != ("ICY 200 OK\r\n" +
		"Content-Type: Test/Content\r\n" +
		"icy-name: TestPlaylist\r\n" +
		"icy-metadata: 1\r\n" +
		"icy-metaint: 5\r\n" +
		"\r\n" +
		`12345` + string(0x03) + `StreamTitle='Test Title - Test Artist';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`67012` + string(0x03) + `StreamTitle='Test Title - Test Artist';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`34567` + string(0x03) + `StreamTitle='Test Title - Test Artist';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`89`) {

		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

	tpl.fp = 0
	drh = NewDefaultRequestHandler(&testPlaylistFactory{tpl}, false, false, "")
	testConn = &testutil.ErrorTestingConnection{}
	testConn.OutErr = 5
	out.Reset()

	drh.defaultServeRequest(testConn, "/testpath", true, 0, "")

	if out.String() != "Serve request path:/testpath Metadata support:true Offset:0\n"+
		"Written bytes: 0\n"+
		"Sending: Test Title - Test Artist\n"+
		"Test writing error\n" {
		t.Error("Unexpected output:", out.String())
		return
	}

	oldTestTitle := testTitle
	testTitle = "A very long title name which should be truncated"
	defer func() {
		testTitle = oldTestTitle
	}()

	oldMaxMetaDataSize := MaxMetaDataSize
	MaxMetaDataSize = 40
	defer func() {
		MaxMetaDataSize = oldMaxMetaDataSize
	}()

	tpl.fp = 0
	drh = NewDefaultRequestHandler(&testPlaylistFactory{tpl}, false, false, "")
	testConn = &testutil.ErrorTestingConnection{}

	drh.defaultServeRequest(testConn, "/testpath", true, 0, "")

	// Meta data is 3*16=48 bytes - text is 40 bytes, padding is 8 bytes

	if testConn.Out.String() != ("ICY 200 OK\r\n" +
		"Content-Type: Test/Content\r\n" +
		"icy-name: TestPlaylist\r\n" +
		"icy-metadata: 1\r\n" +
		"icy-metaint: 5\r\n" +
		"\r\n" +
		`12345` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`67012` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`34567` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`89`) {

		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

	// Test offsets

	tpl.fp = 0
	drh = NewDefaultRequestHandler(&testPlaylistFactory{tpl}, false, false, "")
	testConn = &testutil.ErrorTestingConnection{}

	drh.defaultServeRequest(testConn, "/testpath", true, 7, "")

	// Meta data is 3*16=48 bytes - text is 40 bytes, padding is 8 bytes

	if testConn.Out.String() != ("ICY 200 OK\r\n" +
		"Content-Type: Test/Content\r\n" +
		"icy-name: TestPlaylist\r\n" +
		"icy-metadata: 1\r\n" +
		"icy-metaint: 5\r\n" +
		"\r\n" +
		`01234` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`56789`) {

		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

	tpl.fp = 0
	drh = NewDefaultRequestHandler(&testPlaylistFactory{tpl}, false, false, "")
	testConn = &testutil.ErrorTestingConnection{}

	drh.defaultServeRequest(testConn, "/testpath", true, 2, "")

	// Meta data is 3*16=48 bytes - text is 40 bytes, padding is 8 bytes

	if testConn.Out.String() != ("ICY 200 OK\r\n" +
		"Content-Type: Test/Content\r\n" +
		"icy-name: TestPlaylist\r\n" +
		"icy-metadata: 1\r\n" +
		"icy-metaint: 5\r\n" +
		"\r\n" +
		`34567` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`01234` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`56789`) {

		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

	// Test offset and loops

	tpl.fp = 0
	drh = NewDefaultRequestHandler(&testPlaylistFactory{tpl}, true, false, "")
	testConn = &testutil.ErrorTestingConnection{}
	drh.LoopTimes = 3

	drh.defaultServeRequest(testConn, "/testpath", true, 4, "")

	// Meta data is 3*16=48 bytes - text is 40 bytes, padding is 8 bytes

	if testConn.Out.String() != ("ICY 200 OK\r\n" +
		"Content-Type: Test/Content\r\n" +
		"icy-name: TestPlaylist\r\n" +
		"icy-metadata: 1\r\n" +
		"icy-metaint: 5\r\n" +
		"\r\n" +
		`56701` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`23456` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`78912` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`34567` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`01234` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`56789` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`12345` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`67012` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`34567` + string(0x03) + `StreamTitle='A very long title name wh';` + string([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}) +
		`89`) {

		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

	// Test client close connection

	tpl.fp = 0
	drh = NewDefaultRequestHandler(&testPlaylistFactory{tpl}, false, false, "")
	testConn = &testutil.ErrorTestingConnection{}
	testConn.OutClose = true
	out.Reset()

	drh.defaultServeRequest(testConn, "/testpath", true, 0, "")

	if out.String() != "Serve request path:/testpath Metadata support:true Offset:0\n"+
		"Written bytes: 0\n"+
		"Sending: A very long title name which should be truncated - Test Artist\n"+
		"Could not write to client - closing connection\n" {
		t.Error("Unexpected output:", out.String())
		return
	}

}

func TestRequestHandling(t *testing.T) {

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

	drh := NewDefaultRequestHandler(nil, false, false, "")
	testConn := &testutil.ErrorTestingConnection{}

	// Check normal error return

	drh.HandleRequest(testConn, &testNetError{})

	if out.String() != "Handling request from: <nil>\n"+
		"TestNetError\n" {
		t.Error("Unexpected output:", out.String())
		return
	}

	out.Reset()

	// Test connection writing errors

	testConn = &testutil.ErrorTestingConnection{}
	for i := 0; i < 1600; i++ {
		testConn.In.WriteString("0123456789")
	}
	testConn.InErr = 530

	drh.HandleRequest(testConn, nil)

	if out.String() != "Handling request from: <nil>\n"+
		"Test reading error\n" {
		t.Error("Unexpected output:", out.String())
		return
	}

	out.Reset()
	testConn.In.Reset()
	for i := 0; i < 1600; i++ {
		testConn.In.WriteString("0123456789")
	}
	testConn.InErr = 0

	drh.HandleRequest(testConn, nil)

	if out.String() != "Handling request from: <nil>\n"+
		"Illegal request: Request is too long\n" {
		t.Error("Unexpected output:", out.String())
		return
	}

	out.Reset()
	testConn.In.Reset()
	testConn.In.WriteString("123")
	testConn.InErr = 0

	drh.HandleRequest(testConn, nil)

	if out.String() != "Handling request from: <nil>\n"+
		"Client:<nil> Request:123\r\n\r\n\n"+
		"Invalid request: 123\n" {
		t.Error("Unexpected output:", out.String())
		return
	}

	// Test auth

	drh = NewDefaultRequestHandler(nil, false, false, "web:web")
	testConn = &testutil.ErrorTestingConnection{}

	testConn.In.Reset()
	testConn.In.WriteString(testRequest5)

	// Check normal error return

	drh.HandleRequest(testConn, nil)

	if !strings.Contains(out.String(), "Invalid request (cannot decode authentication)") {
		t.Error("Unexpected output:", out.String())
		return
	}

	out.Reset()

	testConn.In.Reset()
	testConn.In.WriteString(testRequest2)

	// Check normal error return

	drh.HandleRequest(testConn, nil)

	if !strings.Contains(out.String(), "No authentication found") {
		t.Error("Unexpected output:", out.String())
		return
	}

	out.Reset()

	drh = NewDefaultRequestHandler(nil, false, false, "web:web2")
	testConn = &testutil.ErrorTestingConnection{}

	testConn.In.Reset()
	testConn.In.WriteString(testRequest3)

	// Check normal error return

	drh.HandleRequest(testConn, nil)

	if !strings.Contains(out.String(), "Wrong authentication:web:web") {
		t.Error("Unexpected output:", out.String())
		return
	}

	out.Reset()
}

func TestRequestHandler(t *testing.T) {

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

	drh := NewDefaultRequestHandler(nil, false, false, "")
	dds := NewServer(drh.HandleRequest)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		err := dds.Run(testport, &wg)
		if err != nil {
			t.Error(err)
			return
		}
	}()

	wg.Wait()

	rpath := ""
	rmetaDataSupport := false
	roffset := -1
	rauth := ""
	errorChan := make(chan error)

	drh.ServeRequest = func(c net.Conn, path string, metaDataSupport bool, offset int, auth string) {
		rpath = path
		rmetaDataSupport = metaDataSupport
		roffset = offset
		rauth = auth
		errorChan <- nil
	}
	defer func() {
		drh.ServeRequest = drh.defaultServeRequest
	}()

	// Server is now running

	if err := writeSocket([]byte(testRequest)); err != nil {
		t.Error(err)
		return
	}

	<-errorChan

	if rpath != "/mylist" || rmetaDataSupport != true || roffset != 0 || rauth != "" {
		t.Error("Unexpected request decoding result:", rpath, rmetaDataSupport, roffset)
		return
	}

	if err := writeSocket([]byte(testRequest2)); err != nil {
		t.Error(err)
		return
	}

	<-errorChan

	if rpath != "/mylist2" || rmetaDataSupport != true || roffset != 656 || rauth != "" {
		t.Error("Unexpected request decoding result:", rpath, rmetaDataSupport, roffset)
		return
	}

	if err := writeSocket([]byte(testRequest3)); err != nil {
		t.Error(err)
		return
	}

	<-errorChan

	if rpath != "/bach/cello_suite1" || rmetaDataSupport != false || roffset != 0 || rauth != "web:web" {
		t.Error("Unexpected request decoding result:", rpath, rmetaDataSupport, roffset, rauth)
		return
	}

	if err := writeSocket([]byte(testRequest4)); err != nil {
		t.Error(err)
		return
	}

	<-errorChan

	if rpath != "/bach/cello_suite1" || rmetaDataSupport != false || roffset != 0 || rauth != "web:web" {
		t.Error("Unexpected request decoding result:", rpath, rmetaDataSupport, roffset, rauth)
		fmt.Println(testRequest4)
		return
	}

	if err := writeSocket([]byte("\r\n")); err != nil {
		t.Error(err)
		return
	}

	<-errorChan

	if rpath != "/bach/cello_suite1" || rmetaDataSupport != false || roffset != 0 || rauth != "web:web" {
		t.Error("Unexpected request decoding result:", rpath, rmetaDataSupport, roffset, rauth)
		fmt.Println(testRequest4)
		return
	}

	// Shutdown server

	wg.Add(1)

	dds.Shutdown()

	wg.Wait()
}

func writeSocket(req []byte) error {
	conn, err := net.Dial("tcp", testport)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.Write(req)

	return nil
}
