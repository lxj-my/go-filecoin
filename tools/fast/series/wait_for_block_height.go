package series

import (
	"context"
	"io"

	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
)

// WaitForBlockHeight will inspect the chain head and wait till the height is equal to or
// greater than the provide height `bh`
func WaitForBlockHeight(ctx context.Context, client *fast.Filecoin, bh *types.BlockHeight) error {
	for {

		hh, err := GetHeadBlockHeight(ctx, client)
		if err != nil {
			return err
		}

		if hh.GreaterEqual(bh) {
			break
		}

		<-CtxSleepDelay(ctx)
	}

	return nil
}

type MsgInfo struct {
	BlockCid cid.Cid
	MsgCid   cid.Cid
}

// WaitForChainMessage iterates over the chain until the provided function `fn` returns true.
func WaitForChainMessage(ctx context.Context, node *fast.Filecoin, fn func(context.Context, *fast.Filecoin, *types.SignedMessage) (bool, error)) (*MsgInfo, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-CtxSleepDelay(ctx):
			dec, err := node.ChainLs(ctx)
			if err != nil {
				return nil, err
			}

			for {
				var blks []types.Block
				err := dec.Decode(&blks)
				if err != nil {
					if err == io.EOF {
						break
					}

					return nil, err
				}

				for _, blk := range blks {
					msgs, err := node.ShowMessages(ctx, blk.Messages)
					if err != nil {
						return nil, err
					}

					for _, msg := range msgs {
						found, err := fn(ctx, node, msg)
						if err != nil {
							return nil, err
						}

						if found {
							blockCid := blk.Cid()
							msgCid, _ := msg.Cid()

							return &MsgInfo{
								BlockCid: blockCid,
								MsgCid:   msgCid,
							}, nil
						}
					}
				}
			}
		}
	}
}
