package main

import (
	"log"
	"net"
	"strings"

	"github.com/imroc/keda-tencentcloud-clb-scaler/externalscaler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func init() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	registerStringParameter(METRICS_SERVICE_BIND_ADDRESS, ":9000", "The address the gRPRC Metrics Service endpoint binds to")
	registerStringParameter(HEALTH_PROBE_BIND_ADDRESS, ":8081", "The address the probe endpoint binds to")
	registerStringParameter(SECRET_ID, "", "secret id of tencent cloud account")
	registerStringParameter(SECRET_KEY, "", "secret key of tencent cloud account")
}

var rootCmd = &cobra.Command{
	Use:   "tencentcloud-clb-scaler",
	Short: "keda external scaler for tencent cloud clb",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen := viper.GetString(METRICS_SERVICE_BIND_ADDRESS)
		lis, err := net.Listen("tcp", listen)
		if err != nil {
			return err
		}
		grpcServer := grpc.NewServer()
		clbScaler, err := newClbScaler(viper.GetString(REGION), viper.GetString(SECRET_ID), viper.GetString(SECRET_KEY))
		if err != nil {
			return err
		}
		externalscaler.RegisterExternalScalerServer(grpcServer, clbScaler)
		log.Println("listen grpc:", listen)
		if err = grpcServer.Serve(lis); err != nil {
			return err
		}
		return nil
	},
}

func registerStringParameter(name, value, usage string) {
	rootCmd.Flags().String(name, value, usage)
	viper.BindPFlag(name, rootCmd.Flags().Lookup(name))
}
