// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"context"
)

// ContextKey define a type for context.
type ContextKey uint64

// List of valid context key.
const (
	CtxKeyExternalJWT ContextKey = 1 << iota
	CtxKeyInternalJWT
	CtxKeyUID
)

// HandlerFn callback type to handle handshake request.
type HandlerFn func(conn int, req *Frame)

// HandlerAuthFn callback type to handle authentication request.
type HandlerAuthFn func(req *Handshake) (ctx context.Context, err error)

// HandlerClientFn callback type to handle client request.
type HandlerClientFn func(ctx context.Context, conn int)
