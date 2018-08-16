package impl

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeMessage struct {
	api *nodeAPI
}

func newNodeMessage(api *nodeAPI) *nodeMessage {
	return &nodeMessage{api: api}
}

func (api *nodeMessage) Send(ctx context.Context, from, to types.Address, val *types.AttoFIL, method string, params ...interface{}) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&from, nd); err != nil {
		return nil, err
	}

	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, err
	}

	msg, err := node.NewMessageWithNextNonce(ctx, nd, from, to, val, method, encodedParams)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}

func (api *nodeMessage) Wait(ctx context.Context, msgCid *cid.Cid, cb func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt, signature *exec.FunctionSignature) error) error {
	nd := api.api.node

	return nd.ChainMgr.WaitForMessage(ctx, msgCid, func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt) error {
		signature, err := nd.GetSignature(ctx, msg.To, msg.Method)
		if err != nil && err != node.ErrNoMethod {
			return errors.Wrap(err, "unable to determine return type")
		}

		return cb(blk, msg, receipt, signature)
	})
}