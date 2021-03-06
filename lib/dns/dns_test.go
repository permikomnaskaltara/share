// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/shuLhan/share/lib/test"
)

const (
	testServerAddress = "127.0.0.1:5300"
)

var (
	_testServer  *Server        //nolint: gochecknoglobals
	_testHandler *serverHandler //nolint: gochecknoglobals
)

type serverHandler struct {
	responses []*Message
}

func (h *serverHandler) generateResponses() {
	// kilabit.info A
	res := &Message{
		Header: &SectionHeader{
			ID:      1,
			QDCount: 1,
			ANCount: 1,
		},
		Question: &SectionQuestion{
			Name:  []byte("kilabit.info"),
			Type:  QueryTypeA,
			Class: QueryClassIN,
		},
		Answer: []*ResourceRecord{{
			Name:  []byte("kilabit.info"),
			Type:  QueryTypeA,
			Class: QueryClassIN,
			TTL:   3600,
			rdlen: 4,
			Text: &RDataText{
				Value: []byte("127.0.0.1"),
			},
		}},
		Authority:  []*ResourceRecord{},
		Additional: []*ResourceRecord{},
	}

	_, err := res.Pack()
	if err != nil {
		log.Fatal("Pack: ", err)
	}

	h.responses = append(h.responses, res)

	// kilabit.info SOA
	res = &Message{
		Header: &SectionHeader{
			ID:      2,
			QDCount: 1,
			ANCount: 1,
		},
		Question: &SectionQuestion{
			Name:  []byte("kilabit.info"),
			Type:  QueryTypeSOA,
			Class: QueryClassIN,
		},
		Answer: []*ResourceRecord{{
			Name:  []byte("kilabit.info"),
			Type:  QueryTypeSOA,
			Class: QueryClassIN,
			TTL:   3600,
			SOA: &RDataSOA{
				MName:   []byte("kilabit.info"),
				RName:   []byte("admin.kilabit.info"),
				Serial:  20180832,
				Refresh: 3600,
				Retry:   60,
				Expire:  3600,
				Minimum: 3600,
			},
		}},
		Authority:  []*ResourceRecord{},
		Additional: []*ResourceRecord{},
	}

	_, err = res.Pack()
	if err != nil {
		log.Fatal("Pack: ", err)
	}

	h.responses = append(h.responses, res)

	// kilabit.info TXT
	res = &Message{
		Header: &SectionHeader{
			ID:      3,
			QDCount: 1,
			ANCount: 1,
		},
		Question: &SectionQuestion{
			Name:  []byte("kilabit.info"),
			Type:  QueryTypeTXT,
			Class: QueryClassIN,
		},
		Answer: []*ResourceRecord{{
			Name:  []byte("kilabit.info"),
			Type:  QueryTypeTXT,
			Class: QueryClassIN,
			TTL:   3600,
			Text: &RDataText{
				Value: []byte("This is a test server"),
			},
		}},
		Authority:  []*ResourceRecord{},
		Additional: []*ResourceRecord{},
	}

	_, err = res.Pack()
	if err != nil {
		log.Fatal("Pack: ", err)
	}

	h.responses = append(h.responses, res)
}

func (h *serverHandler) ServeDNS(req *Request) {
	var (
		res *Message
		err error
	)

	qname := string(req.Message.Question.Name)
	if qname == "kilabit.info" {
		switch req.Message.Question.Type {
		case QueryTypeA:
			res = h.responses[0]
		case QueryTypeSOA:
			res = h.responses[1]
		case QueryTypeTXT:
			res = h.responses[2]
		}
	}

	// Return empty answer
	if res == nil {
		res = &Message{
			Header: &SectionHeader{
				ID:      req.Message.Header.ID,
				QDCount: 1,
			},
			Question: req.Message.Question,
		}

		_, err = res.Pack()
		if err != nil {
			return
		}
	} else {
		res.SetID(req.Message.Header.ID)
	}

	switch req.Kind {
	case ConnTypeUDP:
		if req.Sender != nil {
			_, err = req.Sender.Send(res, req.UDPAddr)
			if err != nil {
				log.Println("! ServeDNS: Sender.Send: ", err)
			}
		}

	case ConnTypeTCP:
		if req.Sender != nil {
			_, err = req.Sender.Send(res, nil)
			if err != nil {
				log.Println("! ServeDNS: Sender.Send: ", err)
			}
		}

	case ConnTypeDoH:
		if req.ResponseWriter != nil {
			_, err = req.ResponseWriter.Write(res.Packet)
			if err != nil {
				log.Println("! ServeDNS: ResponseWriter.Write: ", err)
			}
			req.ChanResponded <- true
		}
	}
}

func TestMain(m *testing.M) {
	log.SetFlags(log.Lmicroseconds)

	_testHandler = &serverHandler{}

	_testHandler.generateResponses()

	_testServer = &Server{
		Handler: _testHandler,
	}

	serverOptions := &ServerOptions{
		IPAddress:        "127.0.0.1",
		UDPPort:          5300,
		TCPPort:          5300,
		DoHPort:          8443,
		DoHCert:          "testdata/domain.crt",
		DoHCertKey:       "testdata/domain.key",
		DoHAllowInsecure: true,
	}

	go func() {
		err := _testServer.ListenAndServe(serverOptions)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	os.Exit(m.Run())
}

func TestQueryType(t *testing.T) {
	test.Assert(t, "QueryTypeA", QueryTypeA, uint16(1), true)
	test.Assert(t, "QueryTypeTXT", QueryTypeTXT, uint16(16), true)
	test.Assert(t, "QueryTypeAXFR", QueryTypeAXFR, uint16(252), true)
	test.Assert(t, "QueryTypeALL", QueryTypeALL, uint16(255), true)
}
