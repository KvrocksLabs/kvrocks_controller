package storage

import (
	"os"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var testEtcdClient *clientv3.Client

func setup() (err error) {
	testEtcdClient, err = clientv3.New(clientv3.Config{
		Endpoints:   []string{"0.0.0.0:23790"},
		DialTimeout: 5 * time.Second,
	})
	return
}

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		panic("Failed to setup the etcd client: " + err.Error())
	}
	ret := m.Run()
	os.Exit(ret)
}
