// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/shuLhan/share/lib/test"
)

var (
	_testRootRoute = newRootRoute() //nolint: gochecknoglobals
	_testDefMethod string           //nolint: gochecknoglobals
)

func testRouteHandler(t *testing.T, target string) RouteHandler {
	return func(ctx context.Context, req *Request, res *Response) { // nolint: unparam
		test.Assert(t, "routeHandler", target, req.Target, true)
	}
}

func testRootRouteAdd(t *testing.T) {
	cases := []struct {
		desc    string
		method  string
		target  string
		handler RouteHandler
		expErr  error
		exp     *route
	}{{
		desc:    "With invalid method",
		method:  "PUSH",
		target:  "/",
		handler: testRouteHandler(t, "/"),
		expErr:  ErrRouteInvMethod,
	}, {
		desc:    "Without absolute path",
		method:  _testDefMethod,
		target:  ":id/xyz",
		handler: testRouteHandler(t, ":id/xyz"),
		expErr:  ErrRouteInvTarget,
		exp: &route{
			name: "/",
		},
	}, {
		desc:    "With parameter at first path",
		method:  _testDefMethod,
		target:  "/:id/xyz",
		handler: testRouteHandler(t, "/:id/xyz"),
		exp: &route{
			name: "/",
			childs: []*route{{
				name:    "id",
				isParam: true,
				childs: []*route{{
					name:    "xyz",
					handler: testRouteHandler(t, "/:id/xyz"),
				}},
			}},
		},
	}, {
		desc:    "With duplicate parameter",
		method:  _testDefMethod,
		target:  "/:param/abc",
		expErr:  ErrRouteDupParam,
		handler: testRouteHandler(t, "/:id/xyz"),
		exp: &route{
			name: "/",
			childs: []*route{{
				name:    "id",
				isParam: true,
				childs: []*route{{
					name:    "xyz",
					handler: testRouteHandler(t, "/:id/xyz"),
				}},
			}},
		},
	}, {
		desc:    "With handle on root",
		method:  _testDefMethod,
		target:  "/",
		handler: testRouteHandler(t, "/"),
		exp: &route{
			name:    "/",
			handler: testRouteHandler(t, "/"),
			childs: []*route{{
				name:    "id",
				isParam: true,
				childs: []*route{{
					name:    "xyz",
					handler: testRouteHandler(t, "/:id/xyz"),
				}},
			}},
		},
	}, {
		desc:    "With different sub path",
		method:  _testDefMethod,
		target:  "/:id/abc",
		handler: testRouteHandler(t, "/:id/abc"),
		exp: &route{
			name:    "/",
			handler: testRouteHandler(t, "/"),
			childs: []*route{{
				name:    "id",
				isParam: true,
				childs: []*route{{
					name:    "xyz",
					handler: testRouteHandler(t, "/:id/xyz"),
				}, {
					name:    "abc",
					handler: testRouteHandler(t, "/:id/abc"),
				}},
			}},
		},
	}, {
		desc:    "With another parameter at the end",
		method:  _testDefMethod,
		target:  "/:id/abc/def/:000",
		handler: testRouteHandler(t, "/:id/abc/def/:000"),
		exp: &route{
			name:    "/",
			handler: testRouteHandler(t, "/"),
			childs: []*route{{
				name:    "id",
				isParam: true,
				childs: []*route{{
					name:    "xyz",
					handler: testRouteHandler(t, "/:id/xyz"),
				}, {
					name:    "abc",
					handler: testRouteHandler(t, "/:id/abc"),
					childs: []*route{{
						name: "def",
						childs: []*route{{
							name:    "000",
							isParam: true,
							handler: testRouteHandler(t, "/:id/abc/def/:000"),
						}},
					}},
				}},
			}},
		},
	}}

	for _, c := range cases {
		t.Log(c.method + " " + c.target)

		err := _testRootRoute.add(c.method, c.target, c.handler)
		if err != nil {
			test.Assert(t, "err", c.expErr, err, true)
		}

		got := _testRootRoute.getParent(c.method)

		test.Assert(t, "route", fmt.Sprintf("%+v", c.exp), fmt.Sprintf("%+v", got), true)
	}
}

func testRootRouteGet(t *testing.T) {
	cases := []struct {
		desc      string
		method    string
		target    string
		expTarget string
		expParams targetParam
	}{{
		method:    _testDefMethod,
		target:    "/1000/xyz",
		expTarget: "/:id/xyz",
		expParams: targetParam{"id": "1000"},
	}, {
		desc:   "Invalid method",
		method: "PUSH",
		target: "/1000/xyz",
	}, {
		desc:   "Invalid target",
		method: _testDefMethod,
		target: "1000/xy",
	}, {
		desc:   "Invalid target",
		method: _testDefMethod,
		target: "/1000/xy",
	}, {
		method:    _testDefMethod,
		target:    "/",
		expTarget: "/",
		expParams: targetParam{},
	}, {
		method:    _testDefMethod,
		target:    "/333/abc",
		expTarget: "/:id/abc",
		expParams: targetParam{"id": "333"},
	}, {
		method:    _testDefMethod,
		target:    "/333/abc/",
		expTarget: "/:id/abc",
		expParams: targetParam{"id": "333"},
	}, {
		method:    _testDefMethod,
		target:    "/333/abc/def",
		expTarget: "/:id/abc/def",
		expParams: targetParam{"id": "333"},
	}, {
		method: _testDefMethod,
		target: "/333/abc/444",
	}, {
		method: _testDefMethod,
		target: "/333/abc/444/",
	}, {
		method:    _testDefMethod,
		target:    "/333/abc/def/444",
		expTarget: "/:id/abc/def/:000",
		expParams: targetParam{"id": "333", "000": "444"},
	}, {
		method:    _testDefMethod,
		target:    "/333/abc/def/444/",
		expTarget: "/:id/abc/def/:000",
		expParams: targetParam{"id": "333", "000": "444"},
	}, {
		method: _testDefMethod,
		target: "/333/abc/def/444/ghi",
	}}

	for _, c := range cases {
		t.Log(c.method + " " + c.target)

		gotParams, gotHandler := _testRootRoute.get(c.method, c.target)

		test.Assert(t, "params", c.expParams, gotParams, true)

		if gotHandler != nil {
			gotHandler(context.Background(), &Request{Target: c.expTarget}, nil)
		}
	}
}

func TestRootRoute(t *testing.T) {
	_testDefMethod = http.MethodDelete
	t.Run("add/DELETE", testRootRouteAdd)

	_testDefMethod = http.MethodGet
	t.Run("add/GET", testRootRouteAdd)

	_testDefMethod = http.MethodPatch
	t.Run("add/PATCH", testRootRouteAdd)

	_testDefMethod = http.MethodPost
	t.Run("add/POST", testRootRouteAdd)

	_testDefMethod = http.MethodPut
	t.Run("add/PUT", testRootRouteAdd)

	_testDefMethod = http.MethodGet
	t.Run("get/GET", testRootRouteGet)
}
