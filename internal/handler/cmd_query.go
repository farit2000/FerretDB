// Copyright 2021 FerretDB Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/FerretDB/FerretDB/internal/handler/common"
	"github.com/FerretDB/FerretDB/internal/handler/handlererrors"
	"github.com/FerretDB/FerretDB/internal/types"
	"github.com/FerretDB/FerretDB/internal/util/lazyerrors"
	"github.com/FerretDB/FerretDB/internal/util/must"
	"github.com/FerretDB/FerretDB/internal/wire"
)

// CmdQuery implements deprecated OP_QUERY message handling.
func (h *Handler) CmdQuery(ctx context.Context, query *wire.OpQuery) (*wire.OpReply, error) {
	q := query.Query()
	cmd := q.Command()
	collection := query.FullCollectionName

	v, _ := q.Get("speculativeAuthenticate")
	if v != nil && (cmd == "ismaster" || cmd == "isMaster") {
		reply, err := common.IsMaster(ctx, q, h.TCPHost, h.ReplSetName, h.MaxBsonObjectSizeBytes)
		if err != nil {
			return nil, lazyerrors.Error(err)
		}

		replyDoc := must.NotFail(reply.Document())

		document := v.(*types.Document)

		dbName, err := common.GetRequiredParam[string](document, "db")
		if err != nil {
			reply.SetDocument(replyDoc)

			return reply, nil
		}

		doc, err := h.saslStart(ctx, dbName, document)
		if err == nil {
			// speculative authenticate response field is only set if the authentication is successful,
			// for an unsuccessful authentication, saslStart will return an error
			replyDoc.Set("speculativeAuthenticate", doc)
		}

		reply.SetDocument(replyDoc)

		return reply, nil
	}

	if (cmd == "ismaster" || cmd == "isMaster") && strings.HasSuffix(collection, ".$cmd") {
		return common.IsMaster(ctx, query.Query(), h.TCPHost, h.ReplSetName, h.MaxBsonObjectSizeBytes)
	}

	// TODO https://github.com/FerretDB/FerretDB/issues/3008

	// database name typically is either "$external" or "admin"

	if cmd == "saslStart" && strings.HasSuffix(collection, ".$cmd") {
		var emptyPayload types.Binary
		var reply wire.OpReply
		reply.SetDocument(must.NotFail(types.NewDocument(
			"conversationId", int32(1),
			"done", true,
			"payload", emptyPayload,
			"ok", float64(1),
		)))

		return &reply, nil
	}

	return nil, handlererrors.NewCommandErrorMsgWithArgument(
		handlererrors.ErrNotImplemented,
		fmt.Sprintf("CmdQuery: unhandled command %q for collection %q", cmd, collection),
		"OpQuery: "+cmd,
	)
}
