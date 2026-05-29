package feeder

import (
	"context"
	"fmt"
	"sync"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	cmtservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
)

// FeedSubmission is a single pair/price to submit in the per-window batch.
type FeedSubmission struct {
	Pair  string
	Price math.LegacyDec
}

// Broadcaster builds, signs, and broadcasts one tx containing a MsgSubmitFeed
// per submission. The submit loop depends on this interface (testable seam).
type Broadcaster interface {
	SubmitFeeds(ctx context.Context, feeds []FeedSubmission) (txHash string, err error)
}

type codecCfg struct {
	Marshaler         codec.Codec
	InterfaceRegistry codectypes.InterfaceRegistry
	TxConfig          client.TxConfig
}

func newCodec() codecCfg {
	ir := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(ir)
	authtypes.RegisterInterfaces(ir)
	oracletypes.RegisterInterfaces(ir)
	cdc := codec.NewProtoCodec(ir)
	return codecCfg{Marshaler: cdc, InterfaceRegistry: ir, TxConfig: authtx.NewTxConfig(cdc, authtx.DefaultSignModes)}
}

func buildMsgs(feeder, validator string, feeds []FeedSubmission) ([]sdk.Msg, error) {
	msgs := make([]sdk.Msg, 0, len(feeds))
	for _, f := range feeds {
		m := &oracletypes.MsgSubmitFeed{
			Feeder:    feeder,
			Validator: validator,
			Pair:      f.Pair,
			Price:     f.Price.String(),
		}
		if err := m.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("invalid MsgSubmitFeed for %q: %w", f.Pair, err)
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// GRPCBroadcaster signs MsgSubmitFeed batches with the feeder key and
// broadcasts them over gRPC (design §4, Approach A).
type GRPCBroadcaster struct {
	cfg        *Config
	codec      codecCfg
	kr         keyring.Keyring
	conn       *grpc.ClientConn
	feederAddr string
	accNum     uint64
	mu         sync.Mutex
	seq        uint64
	logger     log.Logger
}

// NewGRPCBroadcaster dials the node, loads the feeder key, and primes the
// account number/sequence. Fatal errors here are startup-fatal (design §9).
func NewGRPCBroadcaster(cfg *Config, logger log.Logger) (*GRPCBroadcaster, error) {
	cdc := newCodec()
	kr, err := keyring.New("vertix-feeder", cfg.KeyringBackend, cfg.KeyringDir, nil, cdc.Marshaler)
	if err != nil {
		return nil, fmt.Errorf("open keyring: %w", err)
	}
	rec, err := kr.Key(cfg.KeyName)
	if err != nil {
		return nil, fmt.Errorf("key %q not found: %w", cfg.KeyName, err)
	}
	addr, err := rec.GetAddress()
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(cfg.NodeGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial node: %w", err)
	}
	b := &GRPCBroadcaster{cfg: cfg, codec: cdc, kr: kr, conn: conn, feederAddr: addr.String(), logger: logger}
	if err := b.refreshAccount(context.Background()); err != nil {
		return nil, fmt.Errorf("prime account: %w", err)
	}
	return b, nil
}

func (b *GRPCBroadcaster) refreshAccount(ctx context.Context) error {
	q := authtypes.NewQueryClient(b.conn)
	res, err := q.Account(ctx, &authtypes.QueryAccountRequest{Address: b.feederAddr})
	if err != nil {
		return err
	}
	var acc sdk.AccountI
	if err := b.codec.InterfaceRegistry.UnpackAny(res.Account, &acc); err != nil {
		return err
	}
	b.mu.Lock()
	b.accNum = acc.GetAccountNumber()
	b.seq = acc.GetSequence()
	b.mu.Unlock()
	return nil
}

// LatestHeight returns the node's latest block height (for the submit loop).
func (b *GRPCBroadcaster) LatestHeight(ctx context.Context) (int64, error) {
	c := cmtservice.NewServiceClient(b.conn)
	res, err := c.GetLatestBlock(ctx, &cmtservice.GetLatestBlockRequest{})
	if err != nil {
		return 0, err
	}
	if res.SdkBlock != nil {
		return res.SdkBlock.Header.Height, nil
	}
	return 0, fmt.Errorf("latest block response missing sdk block")
}

// SubmitFeeds builds, signs, and SYNC-broadcasts one tx with the batch.
func (b *GRPCBroadcaster) SubmitFeeds(ctx context.Context, feeds []FeedSubmission) (string, error) {
	msgs, err := buildMsgs(b.feederAddr, b.cfg.Validator, feeds)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	accNum, seq := b.accNum, b.seq
	b.mu.Unlock()

	fees, _ := sdk.ParseCoinsNormalized(b.cfg.Fees)
	gas, _ := math.NewIntFromString(b.cfg.Gas)
	txf := clienttx.Factory{}.
		WithChainID(b.cfg.ChainID).
		WithKeybase(b.kr).
		WithTxConfig(b.codec.TxConfig).
		WithAccountNumber(accNum).
		WithSequence(seq).
		WithFees(fees.String()).
		WithSignMode(signingtypes.SignMode_SIGN_MODE_DIRECT)
	if b.cfg.Gas == "auto" {
		txf = txf.WithSimulateAndExecute(true).WithGasAdjustment(b.cfg.GasAdjustment)
	} else {
		txf = txf.WithGas(gas.Uint64())
	}

	txb, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return "", err
	}
	if err := clienttx.Sign(ctx, txf, b.cfg.KeyName, txb, true); err != nil {
		return "", err
	}
	bz, err := b.codec.TxConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return "", err
	}
	svc := txtypes.NewServiceClient(b.conn)
	res, err := svc.BroadcastTx(ctx, &txtypes.BroadcastTxRequest{TxBytes: bz, Mode: txtypes.BroadcastMode_BROADCAST_MODE_SYNC})
	if err != nil {
		return "", err
	}
	if res.TxResponse.Code != 0 {
		if err := b.refreshAccount(ctx); err == nil {
			b.logger.Debug("resynced account after non-zero code", "code", res.TxResponse.Code)
		}
		return res.TxResponse.TxHash, fmt.Errorf("tx rejected, code %d: %s", res.TxResponse.Code, res.TxResponse.RawLog)
	}
	b.mu.Lock()
	b.seq++
	b.mu.Unlock()
	return res.TxResponse.TxHash, nil
}

// VoteWindow queries the oracle module params for the vote window.
func (b *GRPCBroadcaster) VoteWindow(ctx context.Context) (int64, error) {
	q := oracletypes.NewQueryClient(b.conn)
	res, err := q.Params(ctx, &oracletypes.QueryParamsRequest{})
	if err != nil {
		return 0, err
	}
	return res.Params.VoteWindow, nil
}

// Close releases the gRPC connection.
func (b *GRPCBroadcaster) Close() error { return b.conn.Close() }

var _ Broadcaster = (*GRPCBroadcaster)(nil)
