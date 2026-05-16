package tests

import (
	"flag"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if _, err := os.Stat(getKubeConfig()); err != nil {
		log.WithError(err).Warn("skipping acceptance tests: kubeconfig is not available")
		os.Exit(0)
	}

	os.Exit(m.Run())
}
