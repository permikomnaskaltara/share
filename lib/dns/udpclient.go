// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import (
	"net"
	"sync"
	"time"

	libbytes "github.com/shuLhan/share/lib/bytes"
	"github.com/shuLhan/share/lib/debug"
	libnet "github.com/shuLhan/share/lib/net"
)

//
// UDPClient for DNS with UDP connection.
//
// Any implementation that need to query DNS message in multiple Go routines
// should create one client per routine.
//
type UDPClient struct {
	Timeout time.Duration
	Addr    *net.UDPAddr // Addr contains address of remote nameserver.
	Conn    *net.UDPConn
	sync.Mutex
}

//
// NewUDPClient will create new DNS client with UDP network connection.
//
// The nameserver contains the IP address, not host name, of parent DNS
// server.  Default port is 53, if not set.
//
func NewUDPClient(nameserver string) (cl *UDPClient, err error) {
	network := "udp"

	remoteIP, remotePort, err := libnet.ParseIPPort(nameserver, DefaultPort)
	if err != nil {
		return
	}

	laddr := &net.UDPAddr{IP: nil, Port: 0}
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return
	}

	cl = &UDPClient{
		Timeout: clientTimeout,
		Addr: &net.UDPAddr{
			IP:   remoteIP,
			Port: int(remotePort),
		},
		Conn: conn,
	}

	return
}

//
// RemoteAddr return client remote nameserver address.
//
func (cl *UDPClient) RemoteAddr() string {
	return cl.Addr.String()
}

//
// Close client connection.
//
func (cl *UDPClient) Close() error {
	return cl.Conn.Close()
}

//
// Lookup will query one of the name server with specific type, class, and
// name in synchronous mode.
//
// Name could be a host name for standard query or IP address for inverse
// query.
//
// This function is safe to be used concurrently.
//
func (cl *UDPClient) Lookup(qtype uint16, qclass uint16, qname []byte) (
	*Message, error,
) {
	if cl.Addr == nil || cl.Conn == nil {
		return nil, nil
	}

	msg := NewMessage()

	msg.Header.ID = getNextID()
	msg.Header.QDCount = 1
	msg.Question.Type = qtype
	msg.Question.Class = qclass
	msg.Question.Name = append(msg.Question.Name, qname...)

	_, _ = msg.Pack()

	res, err := cl.Query(msg, cl.Addr)
	if err != nil {
		return nil, err
	}

	return res, nil
}

//
// Query send DNS query to name server "ns" and return the unpacked response.
//
func (cl *UDPClient) Query(msg *Message, ns net.Addr) (*Message, error) {
	if ns == nil {
		ns = cl.Addr
	}

	cl.Lock()

	_, err := cl.Send(msg, ns)
	if err != nil {
		cl.Unlock()
		return nil, err
	}

	res := NewMessage()

	_, err = cl.Recv(res)
	if err != nil {
		cl.Unlock()
		return nil, err
	}

	cl.Unlock()

	err = res.Unpack()
	if err != nil {
		return nil, err
	}

	return res, nil
}

//
// Recv will read DNS message from active connection in client into `msg`.
//
func (cl *UDPClient) Recv(msg *Message) (n int, err error) {
	err = cl.Conn.SetReadDeadline(time.Now().Add(cl.Timeout))
	if err != nil {
		return
	}

	n, _, err = cl.Conn.ReadFromUDP(msg.Packet)
	if err != nil {
		return
	}

	msg.Packet = append(msg.Packet[:0], msg.Packet[:n]...)

	if debug.Value >= 3 {
		libbytes.PrintHex(">>> UDPClient: Recv:", msg.Packet, 8)
	}

	return
}

//
// Send DNS message to name server using active connection in client.
//
// The message packet must already been filled, using Pack().
// The addr parameter must not be nil.
//
func (cl *UDPClient) Send(msg *Message, ns net.Addr) (n int, err error) {
	if ns == nil {
		ns = cl.Addr
	}

	raddr := ns.(*net.UDPAddr)

	err = cl.Conn.SetWriteDeadline(time.Now().Add(cl.Timeout))
	if err != nil {
		return
	}

	n, err = cl.Conn.WriteToUDP(msg.Packet, raddr)

	return
}

//
// SetRemoteAddr set the remote address for sending the packet.
//
func (cl *UDPClient) SetRemoteAddr(addr string) (err error) {
	cl.Addr, err = net.ResolveUDPAddr("udp", addr)
	return
}

//
// SetTimeout set the timeout for sending and receiving packet.
//
func (cl *UDPClient) SetTimeout(t time.Duration) {
	cl.Timeout = t
}
