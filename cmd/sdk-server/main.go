// Copyright 2018 Google LLC All Rights Reserved.
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

// sidecar for the game server that the sdk connects to
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agones.dev/agones/pkg"
	"agones.dev/agones/pkg/client/clientset/versioned"
	"agones.dev/agones/pkg/sdk"
	"agones.dev/agones/pkg/sdkserver"
	"agones.dev/agones/pkg/util/runtime"
	"agones.dev/agones/pkg/util/signals"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultGRPCPort = 9357
	defaultHTTPPort = 9358

	// specifically env vars
	gameServerNameEnv = "GAMESERVER_NAME"
	podNamespaceEnv   = "POD_NAMESPACE"

	// Flags (that can also be env vars)
	localFlag    = "local"
	fileFlag     = "file"
	testFlag     = "test"
	addressFlag  = "address"
	delayFlag    = "delay"
	timeoutFlag  = "timeout"
	grpcPortFlag = "grpc-port"
	httpPortFlag = "http-port"
)

var (
	logger = runtime.NewLoggerWithSource("main")
)

func main() {
	ctlConf := parseEnvFlags()
	logger.WithField("version", pkg.Version).WithField("featureGates", runtime.EncodeFeatures()).
		WithField("ctlConf", ctlConf).Info("Starting sdk sidecar")

	if ctlConf.Delay > 0 {
		logger.Infof("Waiting %d seconds before starting", ctlConf.Delay)
		time.Sleep(time.Duration(ctlConf.Delay) * time.Second)
	}

	stop := signals.NewStopChannel()
	timedStop := make(chan struct{})
	grpcServer := grpc.NewServer()
	// don't graceful stop, because if we get a kill signal
	// then the gameserver is being shut down, and we no longer
	// care about running RPC calls.
	defer grpcServer.Stop()

	mux := gwruntime.NewServeMux()
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", ctlConf.Address, ctlConf.HTTPPort),
		Handler: mux,
	}
	defer httpServer.Close() // nolint: errcheck
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch {
	case ctlConf.IsLocal:
		localSDK, err := registerLocal(grpcServer, ctlConf)
		if err != nil {
			logger.WithError(err).Fatal("Could not start local sdk server")
		}
		defer localSDK.Close()

		if ctlConf.Timeout != 0 {
			go func() {
				time.Sleep(time.Duration(ctlConf.Timeout) * time.Second)
				close(timedStop)
			}()
		}
	case ctlConf.Test != "":
		localSDK, err := registerTestSdkServer(grpcServer, ctlConf)
		if err != nil {
			logger.WithError(err).Fatal("Could not start test sdk server")
		}
		defer localSDK.Close()

		if ctlConf.Timeout != 0 {
			go func() {
				time.Sleep(time.Duration(ctlConf.Timeout) * time.Second)
				close(timedStop)
			}()
		}
	default:
		var config *rest.Config
		config, err := rest.InClusterConfig()
		if err != nil {
			logger.WithError(err).Fatal("Could not create in cluster config")
		}

		var kubeClient *kubernetes.Clientset
		kubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			logger.WithError(err).Fatal("Could not create the kubernetes clientset")
		}

		var agonesClient *versioned.Clientset
		agonesClient, err = versioned.NewForConfig(config)
		if err != nil {
			logger.WithError(err).Fatalf("Could not create the agones api clientset")
		}

		var s *sdkserver.SDKServer
		s, err = sdkserver.NewSDKServer(viper.GetString(gameServerNameEnv),
			viper.GetString(podNamespaceEnv), kubeClient, agonesClient)
		if err != nil {
			logger.WithError(err).Fatalf("Could not start sidecar")
		}

		go func() {
			err := s.Run(ctx.Done())
			if err != nil {
				logger.WithError(err).Fatalf("Could not run sidecar")
			}
		}()
		sdk.RegisterSDKServer(grpcServer, s)
	}

	grpcEndpoint := fmt.Sprintf("%s:%d", ctlConf.Address, ctlConf.GRPCPort)
	go runGrpc(grpcServer, grpcEndpoint)
	go runGateway(ctx, grpcEndpoint, mux, httpServer)

	select {
	case <-stop:
	case <-timedStop:
	}

	logger.Info("shutting down sdk server")
}

func registerLocal(grpcServer *grpc.Server, ctlConf config) (localSDK *sdkserver.LocalSDKServer, err error) {
	filePath := ""
	if ctlConf.LocalFile != "" {
		filePath, err = filepath.Abs(ctlConf.LocalFile)
		if err != nil {
			return
		}

		if _, err = os.Stat(filePath); os.IsNotExist(err) {
			err = errors.Errorf("Could not find file: %s", filePath)
			return
		}
	}

	localSDK, err = sdkserver.NewLocalSDKServer(filePath)
	if err != nil {
		return
	}
	sdk.RegisterSDKServer(grpcServer, localSDK)
	return
}

func registerTestSdkServer(grpcServer *grpc.Server, ctlConf config) (localSDK *sdkserver.LocalSDKServer, err error) {
	localSDK, err = sdkserver.NewLocalSDKServer("")
	if err != nil {
		return
	}
	localSDK.SetTestMode(true)
	localSDK.GenerateUID()
	expectedFuncs := strings.Split(ctlConf.Test, ",")
	localSDK.SetExpectedSequence(expectedFuncs)
	sdk.RegisterSDKServer(grpcServer, localSDK)
	return
}

// runGrpc runs the grpc service
func runGrpc(grpcServer *grpc.Server, grpcEndpoint string) {
	lis, err := net.Listen("tcp", grpcEndpoint)
	if err != nil {
		logger.WithField("grpcEndpoint", grpcEndpoint).Fatal("Could not listen on grpc endpoint")
	}

	logger.WithField("grpcEndpoint", grpcEndpoint).Info("Starting SDKServer grpc service...")
	if err := grpcServer.Serve(lis); err != nil {
		logger.WithError(err).Fatal("Could not serve grpc server")
	}
}

// runGateway runs the grpc-gateway
func runGateway(ctx context.Context, grpcEndpoint string, mux *gwruntime.ServeMux, httpServer *http.Server) {
	conn, err := grpc.DialContext(ctx, grpcEndpoint, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		logger.WithError(err).Fatal("Could not dial grpc server...")
	}

	if err = sdk.RegisterSDKHandler(ctx, mux, conn); err != nil {
		logger.WithError(err).Fatal("Could not register grpc-gateway")
	}

	logger.WithField("httpEndpoint", httpServer.Addr).Info("Starting SDKServer grpc-gateway...")
	if err := httpServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			logger.WithError(err).Info("http server closed")
		} else {
			logger.WithError(err).Fatal("Could not serve http server")
		}
	}
}

// parseEnvFlags parses all the flags and environment variables and returns
// a configuration structure
func parseEnvFlags() config {
	viper.AllowEmptyEnv(true)
	viper.SetDefault(localFlag, false)
	viper.SetDefault(fileFlag, "")
	viper.SetDefault(testFlag, "")
	viper.SetDefault(addressFlag, "localhost")
	viper.SetDefault(delayFlag, 0)
	viper.SetDefault(timeoutFlag, 0)
	viper.SetDefault(grpcPortFlag, defaultGRPCPort)
	viper.SetDefault(httpPortFlag, defaultHTTPPort)
	pflag.Bool(localFlag, viper.GetBool(localFlag),
		"Set this, or LOCAL env, to 'true' to run this binary in local development mode. Defaults to 'false'")
	pflag.StringP(fileFlag, "f", viper.GetString(fileFlag), "Set this, or FILE env var to the path of a local yaml or json file that contains your GameServer resoure configuration")
	pflag.String(addressFlag, viper.GetString(addressFlag), "The Address to bind the server grpcPort to. Defaults to 'localhost'")
	pflag.Int(grpcPortFlag, viper.GetInt(grpcPortFlag), fmt.Sprintf("Port on which to bind the gRPC server. Defaults to %d", defaultGRPCPort))
	pflag.Int(httpPortFlag, viper.GetInt(httpPortFlag), fmt.Sprintf("Port on which to bind the HTTP server. Defaults to %d", defaultHTTPPort))
	pflag.Int(delayFlag, viper.GetInt(delayFlag), "Time to delay (in seconds) before starting to execute main. Useful for tests")
	pflag.Int(timeoutFlag, viper.GetInt(timeoutFlag), "Time of execution (in seconds) before close. Useful for tests")
	pflag.String(testFlag, viper.GetString(testFlag), "List functions which shoud be called during the SDK Conformance test run.")
	runtime.FeaturesBindFlags()
	pflag.Parse()

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	runtime.Must(viper.BindEnv(localFlag))
	runtime.Must(viper.BindEnv(addressFlag))
	runtime.Must(viper.BindEnv(testFlag))
	runtime.Must(viper.BindEnv(gameServerNameEnv))
	runtime.Must(viper.BindEnv(podNamespaceEnv))
	runtime.Must(viper.BindEnv(delayFlag))
	runtime.Must(viper.BindEnv(timeoutFlag))
	runtime.Must(viper.BindEnv(grpcPortFlag))
	runtime.Must(viper.BindEnv(httpPortFlag))
	runtime.Must(viper.BindPFlags(pflag.CommandLine))
	runtime.Must(runtime.FeaturesBindEnv())

	runtime.Must(runtime.ParseFeaturesFromEnv())

	return config{
		IsLocal:   viper.GetBool(localFlag),
		Address:   viper.GetString(addressFlag),
		LocalFile: viper.GetString(fileFlag),
		Delay:     viper.GetInt(delayFlag),
		Timeout:   viper.GetInt(timeoutFlag),
		Test:      viper.GetString(testFlag),
		GRPCPort:  viper.GetInt(grpcPortFlag),
		HTTPPort:  viper.GetInt(httpPortFlag),
	}
}

// config is all the configuration for this program
type config struct {
	Address   string
	IsLocal   bool
	LocalFile string
	Delay     int
	Timeout   int
	Test      string
	GRPCPort  int
	HTTPPort  int
}
