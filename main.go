package main

import (
    "fmt"
    "os"
    "runtime"
    "time"

    "github.com/kejace/etherquery/etherquery"
    "github.com/kejace/go-ethereum/cmd/utils"
    "github.com/kejace/go-ethereum/common"
    "github.com/kejace/go-ethereum/console"
    "github.com/kejace/go-ethereum/eth"
    "github.com/kejace/go-ethereum/logger"
    "github.com/kejace/go-ethereum/logger/glog"
    "github.com/kejace/go-ethereum/metrics"
    "github.com/kejace/go-ethereum/node"
    "github.com/kejace/go-ethereum/params"
    "github.com/kejace/go-ethereum/release"
    "github.com/kejace/go-ethereum/rlp"
    "gopkg.in/urfave/cli.v1"
)

const (
    ClientIdentifier = "Geth-etherquery"
    Version          = "1.5.0-unstable"
    VersionMajor     = 1
    VersionMinor     = 5
    VersionPatch     = 0
    VersionOracle    = "0xfa7b9770ca4cb04296cac84f37736d4041251cdf"
)

var (
    gitCommit       string // set via linker flagg
    nodeNameVersion string
    app             *cli.App
)

func init() {
    if gitCommit == "" {
        nodeNameVersion = Version
    } else {
        nodeNameVersion = Version + "-" + gitCommit[:8]
    }

    app = utils.NewApp(Version, "the go-ethereum command line interface")
    app.Action = geth
    app.Commands = []cli.Command{
        {
            Action: version,
            Name:   "version",
            Usage:  "print ethereum version numbers",
            Description: `
The output of this command is supposed to be machine-readable.
`,
        },
    }

    app.Flags = []cli.Flag{
        utils.IdentityFlag,
        utils.PasswordFileFlag,
        utils.GenesisFileFlag,
        utils.BootnodesFlag,
        utils.DataDirFlag,
        utils.KeyStoreDirFlag,
        utils.BlockchainVersionFlag,
        utils.OlympicFlag,
        utils.FastSyncFlag,
        utils.CacheFlag,
        utils.LightKDFFlag,
        utils.JSpathFlag,
        utils.ListenPortFlag,
        utils.MaxPeersFlag,
        utils.MaxPendingPeersFlag,
        utils.EtherbaseFlag,
        utils.GasPriceFlag,
        utils.MinerThreadsFlag,
        utils.MiningEnabledFlag,
        utils.MiningGPUFlag,
        utils.AutoDAGFlag,
        utils.TargetGasLimitFlag,
        utils.NATFlag,
        utils.NatspecEnabledFlag,
        utils.NoDiscoverFlag,
        utils.NodeKeyFileFlag,
        utils.NodeKeyHexFlag,
        utils.RPCEnabledFlag,
        utils.RPCListenAddrFlag,
        utils.RPCPortFlag,
        utils.RPCApiFlag,
        utils.WSEnabledFlag,
        utils.WSListenAddrFlag,
        utils.WSPortFlag,
        utils.WSApiFlag,
        utils.WSAllowedOriginsFlag,
        utils.IPCDisabledFlag,
        utils.IPCApiFlag,
        utils.IPCPathFlag,
        utils.ExecFlag,
        utils.PreloadJSFlag,
        utils.WhisperEnabledFlag,
        utils.DevModeFlag,
        utils.TestNetFlag,
        utils.VMForceJitFlag,
        utils.VMJitCacheFlag,
        utils.VMEnableJitFlag,
        utils.NetworkIdFlag,
        utils.RPCCORSDomainFlag,
        utils.MetricsEnabledFlag,
        utils.SolcPathFlag,
        utils.GpoMinGasPriceFlag,
        utils.GpoMaxGasPriceFlag,
        utils.GpoFullBlockRatioFlag,
        utils.GpobaseStepDownFlag,
        utils.GpobaseStepUpFlag,
        utils.GpobaseCorrectionFactorFlag,
        utils.ExtraDataFlag,
        cli.StringFlag{
            Name: "gcpproject",
            Usage: "GCP project ID",
            Value: "etherquery",
        },
        cli.StringFlag{
            Name: "dataset",
            Usage: "BigQuery dataset ID",
            Value: "ethereum",
        },
        cli.IntFlag{
            Name: "blocksrev",
            Usage: "blocks service revision number",
        },
    }
    app.Flags = append(app.Flags)

    app.Before = func(ctx *cli.Context) error {
        runtime.GOMAXPROCS(runtime.NumCPU())
        // Start system runtime metrics collection
        go metrics.CollectProcessMetrics(3 * time.Second)

        utils.SetupNetwork(ctx)

        // Deprecation warning.
        if ctx.GlobalIsSet(utils.GenesisFileFlag.Name) {
            common.PrintDepricationWarning("--genesis is deprecated. Switch to use 'geth init /path/to/file'")
        }

        return nil
    }

    app.After = func(ctx *cli.Context) error {
        logger.Flush()
        console.Stdin.Close() // Resets terminal mode.
        return nil
    }
}

func main() {
    if err := app.Run(os.Args); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func makeDefaultExtra() []byte {
    var clientInfo = struct {
        Version   uint
        Name      string
        GoVersion string
        Os        string
    }{uint(VersionMajor<<16 | VersionMinor<<8 | VersionPatch), ClientIdentifier, runtime.Version(), runtime.GOOS}
    extra, err := rlp.EncodeToBytes(clientInfo)
    if err != nil {
        glog.V(logger.Warn).Infoln("error setting canonical miner information:", err)
    }

    if uint64(len(extra)) > params.MaximumExtraDataSize.Uint64() {
        glog.V(logger.Warn).Infoln("error setting canonical miner information: extra exceeds", params.MaximumExtraDataSize)
        glog.V(logger.Debug).Infof("extra: %x\n", extra)
        return nil
    }
    return extra
}

// geth is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func geth(ctx *cli.Context) {
    relconf := release.Config{
        Oracle: common.HexToAddress(VersionOracle),
        Major: uint32(1),
        Minor: uint32(5),
        Patch: uint32(0),
    }
    node := utils.MakeSystemNode(ClientIdentifier, nodeNameVersion, relconf, makeDefaultExtra(), ctx)
    startNode(ctx, node)
    node.Wait()
}

func startNode(ctx *cli.Context, stack *node.Node) {
    eqConfig := &etherquery.EtherQueryConfig{
        Project: ctx.GlobalString("gcpproject"),
        Dataset: ctx.GlobalString("dataset"),
        BatchInterval: time.Second * 15,
        BatchSize: 500,
    }

    if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
        return etherquery.New(eqConfig, ctx);
    }); err != nil {
        utils.Fatalf("Failed to register the etherquery service: %v", err)
    }

    // Start up the node itself
    utils.StartNode(stack)

    var ethereum *eth.Ethereum
    if err := stack.Service(&ethereum); err != nil {
        utils.Fatalf("ethereum service not running: %v", err)
    }

    // Start auxiliary services if enabled
    if ctx.GlobalBool(utils.MiningEnabledFlag.Name) {
        if err := ethereum.StartMining(ctx.GlobalInt(utils.MinerThreadsFlag.Name), ctx.GlobalString(utils.MiningGPUFlag.Name)); err != nil {
            utils.Fatalf("Failed to start mining: %v", err)
        }
    }
}

func version(c *cli.Context) {
    fmt.Println(ClientIdentifier)
    fmt.Println("Version:", Version)
    if gitCommit != "" {
        fmt.Println("Git Commit:", gitCommit)
    }
    fmt.Println("Protocol Versions:", eth.ProtocolVersions)
    fmt.Println("Network Id:", c.GlobalInt(utils.NetworkIdFlag.Name))
    fmt.Println("Go Version:", runtime.Version())
    fmt.Println("OS:", runtime.GOOS)
    fmt.Printf("GOPATH=%s\n", os.Getenv("GOPATH"))
    fmt.Printf("GOROOT=%s\n", runtime.GOROOT())
}
