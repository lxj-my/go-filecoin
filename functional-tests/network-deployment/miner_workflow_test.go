package networkdeployment_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"math/big"
	"testing"

	"github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/fastesting"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	logging.SetDebugLogging()
}

func TestMinerWorkflow(t *testing.T) {
	network := tf.DeploymentTest(t)

	ctx := context.Background()
	ctx, env := fastesting.NewDeploymentEnvironment(ctx, t, network, fast.FilecoinOpts{})
	defer func() {
		err := env.Teardown(ctx)
		require.NoError(t, err)
	}()

	miner := env.RequireNewNodeWithFunds()
	client := env.RequireNewNodeWithFunds()

	t.Run("Verify mining", func(t *testing.T) {
		collateral := big.NewInt(10)
		price := big.NewFloat(0.000000001)
		expiry := big.NewInt(128)

		pparams, err := miner.Protocol(ctx)
		require.NoError(t, err)

		sectorSize := pparams.SupportedSectorSizes[0]

		//
		// Verify that a miner can be created with an ask
		//

		// Create a miner on the miner node
		ask, err := series.CreateStorageMinerWithAsk(ctx, miner, collateral, price, expiry, sectorSize)
		require.NoError(t, err)

		//
		// Verify that a deal can be proposed
		//

		// Connect the client and the miner
		err = series.Connect(ctx, client, miner)
		require.NoError(t, err)

		// Store some data with the miner with the given ask, returns the cid for
		// the imported data, and the deal which was created
		var data bytes.Buffer
		dataReader := io.LimitReader(rand.Reader, 512)
		dataReader = io.TeeReader(dataReader, &data)
		dcid, deal, err := series.ImportAndStoreWithDuration(ctx, client, ask, 32, files.NewReaderFile(dataReader))
		require.NoError(t, err)

		// Wait for the deal to be complete
		_, err = series.WaitForDealState(ctx, client, deal, storagedeal.Complete)
		require.NoError(t, err)

		//
		// Verify that deal shows up on both the miner and the client
		//

		dec, err := client.DealsList(ctx, true, false)
		require.NoError(t, err)

		var dl commands.DealsListResult

		err = dec.Decode(&dl)
		if err == io.EOF {
			require.Equal(t, deal.ProposalCid, dl.ProposalCid)
		} else {
			require.NoError(t, err)
		}

		dec, err = miner.DealsList(ctx, false, true)
		require.NoError(t, err)

		err = dec.Decode(&dl)
		if err == io.EOF {
			require.Equal(t, deal.ProposalCid, dl.ProposalCid)
		} else {
			require.NoError(t, err)
		}

		//
		// Verify that the deal piece can be retrieved
		//

		// Retrieve the stored piece of data
		reader, err := client.RetrievalClientRetrievePiece(ctx, dcid, ask.Miner)
		require.NoError(t, err)

		// Verify that it's all the same
		retrievedData, err := ioutil.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, data.Bytes(), retrievedData)

		//
		// Verify that the deal voucher can be redeemed at the end of the deal
		//

		vouchers, err := client.ClientPayments(ctx, deal.ProposalCid)
		require.NoError(t, err)

		lastVoucher := vouchers[len(vouchers)-1]

		err = series.WaitForBlockHeight(ctx, miner, &lastVoucher.ValidAt)
		require.NoError(t, err)

		var addr address.Address
		err = miner.ConfigGet(ctx, "wallet.defaultAddress", &addr)
		require.NoError(t, err)

		balanceBefore, err := miner.WalletBalance(ctx, addr)
		require.NoError(t, err)

		mcid, err := miner.DealsRedeem(ctx, deal.ProposalCid, fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
		require.NoError(t, err)

		result, err := miner.MessageWait(ctx, mcid)
		require.NoError(t, err)

		balanceAfter, err := miner.WalletBalance(ctx, addr)
		require.NoError(t, err)

		// We add the receipt back to the after balance to "undo" the gas costs, then substract the before balance
		// what is left is the change as a result of redeeming the voucher
		assert.Equal(t, lastVoucher.Amount, balanceAfter.Add(result.Receipt.GasAttoFIL).Sub(balanceBefore))

		//
		// Verify that the miners power has increased
		//

		minfo, err := series.WaitForChainMessage(ctx, miner, func(ctx context.Context, node *fast.Filecoin, msg *types.SignedMessage) (bool, error) {
			if msg.Method == "submitPoSt" && msg.To == ask.Miner {
				return true, nil
			}

			return false, nil
		})
		require.NoError(t, err)

		resp, err := miner.MessageWait(ctx, minfo.MsgCid)
		require.NoError(t, err)
		assert.Equal(t, 0, int(resp.Receipt.ExitCode))

		mpower, err := miner.MinerPower(ctx, ask.Miner)
		require.NoError(t, err)

		// We should have a single sector of power
		assert.Equal(t, &mpower.Power, sectorSize)
	})
}
