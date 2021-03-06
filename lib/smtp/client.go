// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package smtp

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/shuLhan/share/lib/debug"
	libnet "github.com/shuLhan/share/lib/net"
)

//
// Client for SMTP.
//
type Client struct {
	// ServerInfo contains the server information, from the response of
	// EHLO command.
	ServerInfo *ServerInfo

	data  []byte
	buf   bytes.Buffer
	raddr *net.TCPAddr
	conn  net.Conn
}

//
// NewClient create and initialize TCP address to remote SMTP server.  The
// returned client is not connected to the server yet.
//
func NewClient(raddr string) (cl *Client, err error) {
	cl = &Client{
		data: make([]byte, 4096),
	}

	ip, port, err := libnet.ParseIPPort(raddr, 25)
	if err != nil {
		ip, err = lookup(raddr)
		if err != nil {
			return nil, err
		}
		if ip == nil {
			err = fmt.Errorf("client.NewClient: '%s' does not have MX record or IP address", raddr)
			return nil, err
		}

		port = 25
	}

	cl.raddr = &net.TCPAddr{
		IP:   ip,
		Port: int(port),
	}

	fmt.Printf("NewClient: %v\n", cl.raddr)

	return cl, nil
}

//
// Authenticate to server using one of SASL mechanism.
// Currently, the only mechanism available is PLAIN.
//
func (cl *Client) Authenticate(mech Mechanism, username, password string) (
	res *Response, err error,
) {
	var cmd []byte

	switch mech {
	case MechanismPLAIN:
		b := []byte("\x00" + username + "\x00" + password)
		initialResponse := base64.StdEncoding.EncodeToString(b)
		cmd = []byte("AUTH PLAIN " + initialResponse + "\r\n")
	default:
		return nil, fmt.Errorf("client.Authenticate: unknown mechanism")
	}

	return cl.SendCommand(cmd)
}

//
// Connect open a connection to server and return server greeting.
//
func (cl *Client) Connect(insecure bool) (res *Response, err error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure, // nolint: gosec
	}

	cl.conn, err = tls.Dial("tcp", cl.raddr.String(), tlsConfig)
	if err != nil {
		return nil, err
	}

	return cl.recv()
}

//
// Ehlo initialize the SMTP session by sending the EHLO command to server.
// If server does not support EHLO it would be fall back to HELO.
//
// Client MUST use domain name that resolved to DNS A RR (address) (RFC 5321,
// section 2.3.5), or SHOULD use IP address if not possible (RFC 5321, section
// 4.1.4).
//
func (cl *Client) Ehlo(domAddr string) (res *Response, err error) {
	if len(domAddr) == 0 {
		domAddr = getUnicastAddress()
		if domAddr == "" {
			domAddr, err = os.Hostname()
			if err != nil {
				err = errors.New("unable to get unicast address or hostname")
				return nil, err
			}
		} else {
			domAddr = "[" + domAddr + "]"
		}
	}

	req := []byte("EHLO " + domAddr + "\r\n")
	res, err = cl.SendCommand(req)
	if err != nil {
		return nil, err
	}

	if res.Code == StatusOK {
		cl.ServerInfo = NewServerInfo(res)
		return res, nil
	}

	req = []byte("HELO " + domAddr + "\r\n")

	return cl.SendCommand(req)
}

//
// Expand get members of mailing-list.
//
func (cl *Client) Expand(mlist string) (res *Response, err error) {
	if len(mlist) == 0 {
		return nil, nil
	}
	cmd := []byte("EXPN " + mlist + "\r\n")
	return cl.SendCommand(cmd)
}

//
// Help get information on specific command from server.
//
func (cl *Client) Help(cmdName string) (res *Response, err error) {
	cmd := []byte("HELP " + cmdName + "\r\n")
	return cl.SendCommand(cmd)
}

//
// Quit signal the server that the client will close the connection.
//
func (cl *Client) Quit() (res *Response, err error) {
	_, err = cl.conn.Write([]byte("QUIT\r\n"))
	if err == nil {
		res, err = cl.recv()
	}

	_ = cl.conn.Close()

	return res, err
}

//
// MailTx send the mail to server.
// This function is implementation of mail transaction (MAIL, RCPT, and DATA
// commands as described in RFC 5321, section 3.3).
// The MailTx.Data must be internet message format which contains headers and
// content as defined by RFC 5322.
//
// The mail transaction will invoke Ehlo function, only if its never called
// before by client.
//
// On success, it will return the last response, which is the success status
// of data transaction (250).
//
// On fail, it will return response from the failed command with error is
// string combination of command, response code and message.
//
func (cl *Client) MailTx(mail *MailTx) (res *Response, err error) {
	if mail == nil {
		// No operation.
		return nil, nil
	}
	if len(mail.From) == 0 {
		return nil, errors.New("SendMailTx: empty mail 'From' parameter")
	}
	if len(mail.Recipients) == 0 {
		return nil, errors.New("SendMailTx: empty mail 'Recipients' parameter")
	}
	if cl.ServerInfo == nil {
		_, err = cl.Ehlo("localhost")
		if err != nil {
			return nil, err
		}
	}

	cl.buf.Reset()
	fmt.Fprintf(&cl.buf, "MAIL FROM:<%s>\r\n", mail.From)

	res, err = cl.SendCommand(cl.buf.Bytes())
	if err != nil || res.Code != StatusOK {
		err = fmt.Errorf("client.MailTx: MAIL FROM: %d - %s", res.Code, res.Message)
		return res, err
	}

	for _, to := range mail.Recipients {
		cl.buf.Reset()
		fmt.Fprintf(&cl.buf, "RCPT TO:<%s>\r\n", to)

		res, err = cl.SendCommand(cl.buf.Bytes())
		if err != nil || res.Code != StatusOK {
			err = fmt.Errorf("client.MailTx: RCPT TO: %d - %s", res.Code, res.Message)
			return res, err
		}
	}

	cl.buf.Reset()
	cl.buf.WriteString("DATA\r\n")

	res, err = cl.SendCommand(cl.buf.Bytes())
	if err != nil || res.Code != StatusDataReady {
		err = fmt.Errorf("client.MailTx: DATA: %d - %s", res.Code, res.Message)
		return res, err
	}

	cl.buf.Reset()
	cl.buf.Write(mail.Data)
	cl.buf.WriteString("\r\n.\r\n")

	_, err = cl.conn.Write(cl.buf.Bytes())
	if err != nil {
		return nil, err
	}

	res, err = cl.recv()
	if err != nil || res.Code != StatusOK {
		err = fmt.Errorf("client.MailTx: Message: %d - %s", res.Code, res.Message)
	}

	return res, err
}

//
// SendCommand send any custom command to server.
//
func (cl *Client) SendCommand(cmd []byte) (res *Response, err error) {
	_, err = cl.conn.Write(cmd)
	if err != nil {
		return nil, err
	}

	return cl.recv()
}

//
// Verify send the VRFY command to server to check if mailbox is exist.
//
func (cl *Client) Verify(mailbox string) (res *Response, err error) {
	if len(mailbox) == 0 {
		return nil, nil
	}
	cmd := []byte("VRFY " + mailbox + "\r\n")
	return cl.SendCommand(cmd)
}

//
// The remote address can be a hostname or IP address with port.
// If its a host name, the client will try to lookup the MX record first, if
// its fail it will resolve the IP address and use it.
//
func lookup(address string) (ip net.IP, err error) {
	mxs, err := net.LookupMX(address)
	if err == nil && len(mxs) > 0 {
		// Select the lowest MX preferences.
		pref := uint16(65535)
		for _, mx := range mxs {
			if mx.Pref < pref {
				address = mx.Host
			}
		}
	}

	ips, err := net.LookupIP(address)
	if err != nil {
		return nil, err
	}

	if len(ips) > 0 {
		return ips[0], nil
	}

	return nil, nil
}

//
// getUnicastAddress return the local unicast address other than localhost.
//
func getUnicastAddress() (saddr string) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		saddr = addr.String()
		if strings.HasSuffix(saddr, "127") {
			continue
		}
		return saddr
	}
	return ""
}

//
// recv read and parse the response from server.
//
func (cl *Client) recv() (res *Response, err error) {
	cl.buf.Reset()

	for {
		n, err := cl.conn.Read(cl.data)
		if n > 0 {
			_, _ = cl.buf.Write(cl.data[:n])
		}
		if err != nil {
			return nil, err
		}
		if n == cap(cl.data) {
			continue
		}
		break
	}

	if debug.Value > 0 {
		fmt.Printf("Client.recv: %s\n", cl.buf.Bytes())
	}

	res, err = NewResponse(cl.buf.Bytes())
	if err != nil {
		return nil, err
	}

	return res, nil
}
