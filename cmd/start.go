package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/open-feature/flagd/pkg/eval"
	"github.com/open-feature/flagd/pkg/runtime"
	"github.com/open-feature/flagd/pkg/service"
	"github.com/open-feature/flagd/pkg/sync"
	"github.com/spf13/cobra"
)

var (
	serviceProvider   string
	syncProvider      string
	evaluator      		string
	uri               string
	httpServicePort   int32
	socketServicePath string
	bearerToken       string
	remoteSyncURL     string
)

func findService(name string) (service.IService, error) {
	registeredServices := map[string]service.IService{
		"http": &service.HttpService{
			HttpServiceConfiguration: &service.HttpServiceConfiguration{
				Port: int32(httpServicePort),
			},
		},
	}
	if v, ok := registeredServices[name]; !ok {
		return nil, errors.New("no service-provider set")
	} else {
		log.Debugf("Using %s service-provider\n", name)
		return v, nil
	}
}

func findSync(name string) (sync.ISync, error) {
	registeredSync := map[string]sync.ISync{
		"filepath": &sync.FilePathSync{
			URI: uri,
		},
		"remote": &sync.HttpSync{
			URI:         uri,
			BearerToken: bearerToken,
			Client: &http.Client{
				Timeout: time.Second * 10,
			},
		},
	}
	if v, ok := registeredSync[name]; !ok {
		return nil, errors.New("no sync-provider set")
	} else {
		log.Debugf("Using %s sync-provider\n", name)
		return v, nil
	}
}

func findEvaluator(name string) (eval.IEvaluator, error) {
	registeredEvaluators := map[string]eval.IEvaluator{
		"json": &eval.JsonEvaluator{},
	}
	if v, ok := registeredEvaluators[name]; !ok {
		return nil, errors.New("no evaluator set")
	} else {
		log.Debugf("Using %s evaluator\n", name)
		return v, nil
	}
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start flagd",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		// Configure service-provider impl------------------------------------------
		var serviceImpl service.IService
		if foundService, err := findService(serviceProvider); err != nil {
			log.Errorf("Unable to find service '%s'", serviceProvider)
			return
		} else {
			serviceImpl = foundService
		}

		// Configure sync-provider impl--------------------------------------------
		var syncImpl sync.ISync
		if foundSync, err := findSync(syncProvider); err != nil {
			log.Errorf("Unable to find sync '%s'", syncProvider)
			return
		} else {
			syncImpl = foundSync
		}

		// Configure evaluator-provider impl------------------------------------------
		var evalImpl eval.IEvaluator
		if foundEval, err := findEvaluator(evaluator); err != nil {
			log.Errorf("Unable to find evaluator '%s'", evaluator)
			return
		} else {
			evalImpl = foundEval
		}

		// Serve ------------------------------------------------------------------
		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error)
		go func() {
			errc <- func() error {
				c := make(chan os.Signal)
				signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
				return fmt.Errorf("%s", <-c)
			}()
		}()

		go runtime.Start(syncImpl, serviceImpl, evalImpl, ctx)

		err := <-errc
		if err != nil {
			cancel()
			log.Printf(err.Error())
		}
	},
}

func init() {
	startCmd.Flags().Int32VarP(&httpServicePort, "port", "p", 8080, "Port to listen on")
	startCmd.Flags().StringVarP(&socketServicePath, "socketpath", "d", "/tmp/flagd.sock", "flagd socket path")
	startCmd.Flags().StringVarP(&serviceProvider, "service-provider", "s", "http", "Set a serve provider e.g. http or socket")
	startCmd.Flags().StringVarP(&syncProvider, "sync-provider", "y", "filepath", "Set a sync provider e.g. filepath or remote")
startCmd.Flags().StringVarP(&evaluator, "evaluator", "e", "json", "Set an evaluator e.g. json")
	startCmd.Flags().StringVarP(&uri, "uri", "f", "", "Set a sync provider uri to read data from this can be a filepath or url")
	startCmd.Flags().StringVarP(&bearerToken, "bearer-token", "b", "", "Set a bearer token to use for remote sync")

	startCmd.MarkFlagRequired("uri")
	rootCmd.AddCommand(startCmd)

}
