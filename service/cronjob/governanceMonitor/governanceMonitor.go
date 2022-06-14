package main

import (
	"flag"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/zecrey-labs/zecrey-eth-rpc/_rpc"
	"github.com/zecrey-labs/zecrey-legend/service/cronjob/governanceMonitor/governanceMonitor"
	"github.com/zecrey-labs/zecrey-legend/service/cronjob/governanceMonitor/internal/config"
	"github.com/zecrey-labs/zecrey-legend/service/cronjob/governanceMonitor/internal/logic"
	"github.com/zecrey-labs/zecrey-legend/service/cronjob/governanceMonitor/internal/server"
	"github.com/zecrey-labs/zecrey-legend/service/cronjob/governanceMonitor/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

var configFile = flag.String("f",
	"D:\\Projects\\mygo\\src\\Zecrey\\SherLzp\\zecrey-legend\\service\\cronjob\\governanceMonitor\\etc\\local.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.MustSetup(c.LogConf)
	ctx := svc.NewServiceContext(c)
	srv := server.NewGovernanceMonitorServer(ctx)

	GovernanceContractAddress, err := ctx.SysConfigModel.GetSysconfigByName(c.ChainConfig.GovernanceContractAddrSysConfigName)

	if err != nil {
		logx.Severef("[governanceMonitor] fatal error, cannot fetch ZecreyLegendContractAddr from sysConfig, err: %s, SysConfigName: %s",
			err.Error(), c.ChainConfig.GovernanceContractAddrSysConfigName)
		panic(err)
	}

	NetworkRpc, err := ctx.SysConfigModel.GetSysconfigByName(c.ChainConfig.NetworkRPCSysConfigName)
	if err != nil {
		logx.Severef("[governanceMonitor] fatal error, cannot fetch NetworkRPC from sysConfig, err: %s, SysConfigName: %s",
			err.Error(), c.ChainConfig.NetworkRPCSysConfigName)
		panic(err)
	}

	logx.Infof("[governanceMonitor] ChainName: %s, GovernanceContractAddress: %s, NetworkRpc: %s",
		c.ChainConfig.GovernanceContractAddrSysConfigName,
		GovernanceContractAddress.Value,
		NetworkRpc.Value)

	// load client
	cli, err := _rpc.NewClient(NetworkRpc.Value)
	if err != nil {
		panic(err)
	}

	// new cron
	cronjob := cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cron.DiscardLogger),
	))

	_, err = cronjob.AddFunc("@every 10s", func() {
		logx.Info("========================= start monitor blocks =========================")
		err := logic.MonitorGovernanceContract(
			cli,
			c.ChainConfig.StartL1BlockHeight, c.ChainConfig.PendingBlocksCount, c.ChainConfig.MaxHandledBlocksCount,
			GovernanceContractAddress.Value,
			ctx.L1BlockMonitorModel,
			ctx.SysConfigModel,
			ctx.L2AssetInfoModel,
		)
		if err != nil {
			logx.Error("[logic.MonitorGovernanceContract main] unable to run:", err)
		}
		logx.Info("========================= end monitor blocks =========================")
	})
	if err != nil {
		panic(err)
	}
	cronjob.Start()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		governanceMonitor.RegisterGovernanceMonitorServer(grpcServer, srv)
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
