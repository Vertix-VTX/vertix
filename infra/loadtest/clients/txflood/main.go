package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

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
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/spf13/viper"

	oracletypes "github.com/vertix-network/vertix/x/oracle/types"
	rwatypes "github.com/vertix-network/vertix/x/rwa/types"
)

type mixConfig struct {
	Run struct {
		DurationSec   int     `mapstructure:"duration_sec"`
		RampRates     []int   `mapstructure:"ramp_rates"`
		RPC           string  `mapstructure:"rpc"`
		GRPC          string  `mapstructure:"grpc"`
		ChainID       string  `mapstructure:"chain_id"`
		Fees          string  `mapstructure:"fees"`
		Gas           string  `mapstructure:"gas"`
		GasAdjustment float64 `mapstructure:"gas_adjustment"`
	} `mapstructure:"run"`
	Mix struct {
		BankSend     float64 `mapstructure:"bank_send"`
		OracleSubmit float64 `mapstructure:"oracle_submit"`
		RwaRegister  float64 `mapstructure:"rwa_register"`
	} `mapstructure:"mix"`
	Accounts struct {
		BankKeys   []string `mapstructure:"bank_keys"`
		OracleKeys []string `mapstructure:"oracle_keys"`
		RwaKeys    []string `mapstructure:"rwa_keys"`
		FaucetKey  string   `mapstructure:"faucet_key"`
	} `mapstructure:"accounts"`
	Oracle struct {
		FeederValidators map[string]string `mapstructure:"feeder_validators"`
		DefaultPair      string            `mapstructure:"default_pair"`
		DefaultPrice     string            `mapstructure:"default_price"`
	} `mapstructure:"oracle"`
	RWA struct {
		OraclePair string `mapstructure:"oracle_pair"`
		Bond       string `mapstructure:"bond"`
		NamePrefix string `mapstructure:"name_prefix"`
	} `mapstructure:"rwa"`
}

type codecCfg struct {
	Marshaler         codec.Codec
	InterfaceRegistry codectypes.InterfaceRegistry
	TxConfig          client.TxConfig
}

type accountState struct {
	mu      sync.Mutex
	accNum  uint64
	seq     uint64
	address string
}

type floodStats struct {
	Submitted int64            `json:"submitted"`
	Success   int64            `json:"success"`
	Failed    int64            `json:"failed"`
	MixCounts map[string]int64 `json:"mix_counts"`
	latMu     sync.Mutex
	latencies selectedLatencies
}

type selectedLatencies struct {
	ms []float64
}

type rampSummary struct {
	ChainID      string             `json:"chain_id"`
	RateTarget   int                `json:"rate_target"`
	DurationSec  int                `json:"duration_sec"`
	Submitted    int64              `json:"submitted"`
	Success      int64              `json:"success"`
	Failed       int64              `json:"failed"`
	SustainedTPS float64            `json:"sustained_tps"`
	RejectRate   float64            `json:"reject_rate"`
	LatencyMS    map[string]float64 `json:"latency_ms"`
	MixCounts    map[string]int64   `json:"mix_counts"`
}

type runSummary struct {
	Ramps      []rampSummary `json:"ramps"`
	PeakTPS    float64       `json:"peak_sustained_tps"`
	BarTPS     float64       `json:"bar_tps_80pct"`
	FinishedAt string        `json:"finished_at"`
}

func newCodec() codecCfg {
	ir := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(ir)
	authtypes.RegisterInterfaces(ir)
	banktypes.RegisterInterfaces(ir)
	oracletypes.RegisterInterfaces(ir)
	rwatypes.RegisterInterfaces(ir)
	cdc := codec.NewProtoCodec(ir)
	return codecCfg{Marshaler: cdc, InterfaceRegistry: ir, TxConfig: authtx.NewTxConfig(cdc, authtx.DefaultSignModes)}
}

func loadConfig(path string) (mixConfig, error) {
	var cfg mixConfig
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return cfg, err
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
	}
	if cfg.Run.DurationSec <= 0 {
		cfg.Run.DurationSec = 600
	}
	if len(cfg.Run.RampRates) == 0 {
		cfg.Run.RampRates = []int{50, 100, 150}
	}
	if cfg.Oracle.DefaultPair == "" {
		cfg.Oracle.DefaultPair = "BTC:USD"
	}
	if cfg.Oracle.DefaultPrice == "" {
		cfg.Oracle.DefaultPrice = "65000.00"
	}
	if cfg.RWA.OraclePair == "" {
		cfg.RWA.OraclePair = "BTC:USD"
	}
	if cfg.RWA.Bond == "" {
		cfg.RWA.Bond = "10000000000"
	}
	if cfg.RWA.NamePrefix == "" {
		cfg.RWA.NamePrefix = "loadtest"
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("config", "infra/loadtest/mix.toml", "path to mix.toml")
	keyringDir := flag.String("keyring-dir", "", "keyring directory")
	keyringBackend := flag.String("keyring-backend", "test", "keyring backend")
	grpcAddr := flag.String("grpc", "", "override gRPC address")
	rateOverride := flag.Int("rate", 0, "single ramp rate override (tx/s); 0 = use ramp_rates from config")
	durationOverride := flag.Int("duration", 0, "override total duration seconds")
	flag.Parse()

	if *keyringDir == "" {
		fmt.Fprintln(os.Stderr, "txflood: --keyring-dir is required")
		os.Exit(2)
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "txflood: load config: %v\n", err)
		os.Exit(1)
	}
	if *grpcAddr != "" {
		cfg.Run.GRPC = *grpcAddr
	}
	if cfg.Run.GRPC == "" {
		fmt.Fprintln(os.Stderr, "txflood: grpc address required")
		os.Exit(2)
	}

	cdc := newCodec()
	kr, err := keyring.New("txflood", *keyringBackend, *keyringDir, nil, cdc.Marshaler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "txflood: keyring: %v\n", err)
		os.Exit(1)
	}

	conn, err := grpc.NewClient(cfg.Run.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "txflood: dial grpc: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx := context.Background()
	accounts, err := primeAccounts(ctx, kr, conn, cdc, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "txflood: prime accounts: %v\n", err)
		os.Exit(1)
	}
	feederValopers, err := resolveFeederValidators(kr, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "txflood: oracle validators: %v\n", err)
		os.Exit(1)
	}

	rates := cfg.Run.RampRates
	totalDur := cfg.Run.DurationSec
	if *rateOverride > 0 {
		rates = []int{*rateOverride}
	}
	if *durationOverride > 0 {
		totalDur = *durationOverride
	}
	segDur := totalDur / len(rates)
	if segDur < 1 {
		segDur = 1
	}

	summary := runSummary{FinishedAt: time.Now().UTC().Format(time.RFC3339)}
	var peak float64

	for _, rate := range rates {
		ramp := runRamp(ctx, conn, kr, cdc, cfg, accounts, feederValopers, rate, segDur)
		summary.Ramps = append(summary.Ramps, ramp)
		if ramp.SustainedTPS > peak {
			peak = ramp.SustainedTPS
		}
		enc, _ := json.Marshal(ramp)
		fmt.Println(string(enc))
	}

	summary.PeakTPS = peak
	summary.BarTPS = peak * 0.8
	enc, err := json.Marshal(summary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "txflood: encode summary: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(enc))
}

func primeAccounts(
	ctx context.Context,
	kr keyring.Keyring,
	conn *grpc.ClientConn,
	cdc codecCfg,
	cfg mixConfig,
) (map[string]*accountState, error) {
	names := uniqueKeys(cfg)
	out := make(map[string]*accountState, len(names))
	q := authtypes.NewQueryClient(conn)
	for _, name := range names {
		rec, err := kr.Key(name)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", name, err)
		}
		addr, err := rec.GetAddress()
		if err != nil {
			return nil, err
		}
		res, err := q.Account(ctx, &authtypes.QueryAccountRequest{Address: addr.String()})
		if err != nil {
			return nil, fmt.Errorf("query account %q: %w", name, err)
		}
		var acc sdk.AccountI
		if err := cdc.InterfaceRegistry.UnpackAny(res.Account, &acc); err != nil {
			return nil, err
		}
		out[name] = &accountState{
			accNum:  acc.GetAccountNumber(),
			seq:     acc.GetSequence(),
			address: addr.String(),
		}
	}
	return out, nil
}

func uniqueKeys(cfg mixConfig) []string {
	seen := map[string]struct{}{}
	var keys []string
	add := func(k string) {
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for _, k := range cfg.Accounts.BankKeys {
		add(k)
	}
	for _, k := range cfg.Accounts.OracleKeys {
		add(k)
	}
	for _, k := range cfg.Accounts.RwaKeys {
		add(k)
	}
	for _, k := range cfg.Oracle.FeederValidators {
		add(k)
	}
	return keys
}

func resolveFeederValidators(kr keyring.Keyring, cfg mixConfig) (map[string]string, error) {
	out := make(map[string]string, len(cfg.Oracle.FeederValidators))
	for feeder, founder := range cfg.Oracle.FeederValidators {
		rec, err := kr.Key(founder)
		if err != nil {
			return nil, fmt.Errorf("founder key %q for feeder %q: %w", founder, feeder, err)
		}
		addr, err := rec.GetAddress()
		if err != nil {
			return nil, err
		}
		valAddr := sdk.ValAddress(addr.Bytes()).String()
		out[feeder] = valAddr
	}
	return out, nil
}

func runRamp(
	ctx context.Context,
	conn *grpc.ClientConn,
	kr keyring.Keyring,
	cdc codecCfg,
	cfg mixConfig,
	accounts map[string]*accountState,
	feederValopers map[string]string,
	rate int,
	durationSec int,
) rampSummary {
	stats := &floodStats{MixCounts: map[string]int64{
		"bank_send": 0, "oracle_submit": 0, "rwa_register": 0,
	}}

	var assetCounter atomic.Int64
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec)*time.Second)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var wg sync.WaitGroup
loop:
	for {
		select {
		case <-runCtx.Done():
			break loop
		case <-ticker.C:
			for i := 0; i < rate; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					sendOne(runCtx, conn, kr, cdc, cfg, accounts, feederValopers, stats, rng, &assetCounter)
				}()
			}
		}
	}
	wg.Wait()

	lat := stats.latencies.ms
	sort.Float64s(lat)
	summary := rampSummary{
		ChainID:     cfg.Run.ChainID,
		RateTarget:  rate,
		DurationSec: durationSec,
		Submitted:   stats.Submitted,
		Success:     stats.Success,
		Failed:      stats.Failed,
		MixCounts:   stats.MixCounts,
		LatencyMS: map[string]float64{
			"p50": percentile(lat, 50),
			"p99": percentile(lat, 99),
		},
	}
	if durationSec > 0 {
		summary.SustainedTPS = float64(stats.Success) / float64(durationSec)
	}
	if stats.Submitted > 0 {
		summary.RejectRate = float64(stats.Failed) / float64(stats.Submitted)
	}
	return summary
}

func sendOne(
	ctx context.Context,
	conn *grpc.ClientConn,
	kr keyring.Keyring,
	cdc codecCfg,
	cfg mixConfig,
	accounts map[string]*accountState,
	feederValopers map[string]string,
	stats *floodStats,
	rng *rand.Rand,
	assetCounter *atomic.Int64,
) {
	atomic.AddInt64(&stats.Submitted, 1)
	kind := pickMix(cfg, rng)
	stats.latMu.Lock()
	stats.MixCounts[kind]++
	stats.latMu.Unlock()

	msg, keyName, err := buildMsg(kind, cfg, accounts, feederValopers, rng, assetCounter)
	if err != nil {
		atomic.AddInt64(&stats.Failed, 1)
		return
	}

	start := time.Now()
	hash, err := broadcast(ctx, conn, kr, cdc, cfg, accounts[keyName], keyName, msg)
	if err != nil {
		atomic.AddInt64(&stats.Failed, 1)
		return
	}
	if err := waitInclusion(ctx, conn, hash, 30*time.Second); err != nil {
		atomic.AddInt64(&stats.Failed, 1)
		return
	}
	atomic.AddInt64(&stats.Success, 1)
	stats.latMu.Lock()
	stats.latencies.ms = append(stats.latencies.ms, float64(time.Since(start).Milliseconds()))
	stats.latMu.Unlock()
}

func pickMix(cfg mixConfig, rng *rand.Rand) string {
	r := rng.Float64()
	if r < cfg.Mix.BankSend {
		return "bank_send"
	}
	if r < cfg.Mix.BankSend+cfg.Mix.OracleSubmit {
		return "oracle_submit"
	}
	return "rwa_register"
}

func buildMsg(
	kind string,
	cfg mixConfig,
	accounts map[string]*accountState,
	feederValopers map[string]string,
	rng *rand.Rand,
	assetCounter *atomic.Int64,
) (sdk.Msg, string, error) {
	switch kind {
	case "bank_send":
		if len(cfg.Accounts.BankKeys) < 2 {
			return nil, "", fmt.Errorf("need >=2 bank keys")
		}
		from := cfg.Accounts.BankKeys[rng.Intn(len(cfg.Accounts.BankKeys))]
		to := cfg.Accounts.BankKeys[rng.Intn(len(cfg.Accounts.BankKeys))]
		for to == from && len(cfg.Accounts.BankKeys) > 1 {
			to = cfg.Accounts.BankKeys[rng.Intn(len(cfg.Accounts.BankKeys))]
		}
		fromAcc := accounts[from]
		return banktypes.NewMsgSend(
			sdk.MustAccAddressFromBech32(fromAcc.address),
			sdk.MustAccAddressFromBech32(accounts[to].address),
			sdk.NewCoins(sdk.NewInt64Coin("uvtx", 1000)),
		), from, nil

	case "oracle_submit":
		if len(cfg.Accounts.OracleKeys) == 0 {
			return nil, "", fmt.Errorf("no oracle keys")
		}
		feeder := cfg.Accounts.OracleKeys[rng.Intn(len(cfg.Accounts.OracleKeys))]
		valoper, ok := feederValopers[feeder]
		if !ok {
			return nil, "", fmt.Errorf("no validator for feeder %q", feeder)
		}
		m := &oracletypes.MsgSubmitFeed{
			Feeder:    accounts[feeder].address,
			Validator: valoper,
			Pair:      cfg.Oracle.DefaultPair,
			Price:     cfg.Oracle.DefaultPrice,
		}
		if err := m.ValidateBasic(); err != nil {
			return nil, "", err
		}
		return m, feeder, nil

	case "rwa_register":
		if len(cfg.Accounts.RwaKeys) == 0 {
			return nil, "", fmt.Errorf("no rwa keys")
		}
		issuer := cfg.Accounts.RwaKeys[rng.Intn(len(cfg.Accounts.RwaKeys))]
		n := assetCounter.Add(1)
		assetID := fmt.Sprintf("%s-%d-%d", cfg.RWA.NamePrefix, time.Now().UnixNano(), n)
		if len(assetID) > 43 {
			assetID = assetID[:43]
		}
		m := &rwatypes.MsgRegisterAsset{
			Issuer:      accounts[issuer].address,
			AssetId:     assetID,
			Name:        "Load Test Asset",
			Description: "Phase 9 load harness",
			OraclePair:  cfg.RWA.OraclePair,
			Bond:        cfg.RWA.Bond,
		}
		if err := m.ValidateBasic(); err != nil {
			return nil, "", err
		}
		return m, issuer, nil
	default:
		return nil, "", fmt.Errorf("unknown mix kind %q", kind)
	}
}

func broadcast(
	ctx context.Context,
	conn *grpc.ClientConn,
	kr keyring.Keyring,
	cdc codecCfg,
	cfg mixConfig,
	acct *accountState,
	keyName string,
	msg sdk.Msg,
) (string, error) {
	acct.mu.Lock()
	accNum, seq := acct.accNum, acct.seq
	acct.mu.Unlock()

	fees, err := sdk.ParseCoinsNormalized(cfg.Run.Fees)
	if err != nil {
		return "", err
	}
	gas, ok := math.NewIntFromString(cfg.Run.Gas)
	if !ok {
		return "", fmt.Errorf("invalid gas %q", cfg.Run.Gas)
	}
	txf := clienttx.Factory{}.
		WithChainID(cfg.Run.ChainID).
		WithKeybase(kr).
		WithTxConfig(cdc.TxConfig).
		WithAccountNumber(accNum).
		WithSequence(seq).
		WithFees(fees.String()).
		WithSignMode(signingtypes.SignMode_SIGN_MODE_DIRECT).
		WithGas(gas.Uint64())

	txb, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		return "", err
	}
	if err := clienttx.Sign(ctx, txf, keyName, txb, true); err != nil {
		return "", err
	}
	bz, err := cdc.TxConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return "", err
	}

	svc := txtypes.NewServiceClient(conn)
	res, err := svc.BroadcastTx(ctx, &txtypes.BroadcastTxRequest{
		TxBytes: bz,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	})
	if err != nil {
		return "", err
	}
	if res.TxResponse.Code != 0 {
		return res.TxResponse.TxHash, fmt.Errorf("code %d: %s", res.TxResponse.Code, res.TxResponse.RawLog)
	}

	acct.mu.Lock()
	acct.seq++
	acct.mu.Unlock()
	return res.TxResponse.TxHash, nil
}

func waitInclusion(ctx context.Context, conn *grpc.ClientConn, hash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	txClient := txtypes.NewServiceClient(conn)
	cmtClient := cmtservice.NewServiceClient(conn)

	var startHeight int64
	for time.Now().Before(deadline) {
		res, err := txClient.GetTx(ctx, &txtypes.GetTxRequest{Hash: hash})
		if err == nil && res.TxResponse != nil && res.TxResponse.Height > 0 {
			return nil
		}
		if startHeight == 0 {
			b, err := cmtClient.GetLatestBlock(ctx, &cmtservice.GetLatestBlockRequest{})
			if err == nil && b.SdkBlock != nil {
				startHeight = b.SdkBlock.Header.Height
			}
		} else {
			b, err := cmtClient.GetLatestBlock(ctx, &cmtservice.GetLatestBlockRequest{})
			if err == nil && b.SdkBlock != nil && b.SdkBlock.Header.Height > startHeight+2 {
				return fmt.Errorf("tx not found after blocks advanced")
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("inclusion timeout for %s", hash)
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	idx := int(float64(len(sorted)-1) * (p / 100.0))
	return sorted[idx]
}
