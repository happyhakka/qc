package api

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"

	"go.etcd.io/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/naming"
)

const (
	EV_ETCD_ADDR      string = "etcd_addr"
	EV_ETCD_CA_FILE   string = "etcd_ca_file"
	EV_ETCD_CERT_FILE string = "etcd_cert_file"
	EV_ETCD_KEY_FILE  string = "etcd_key_file"
)

//实现 grpc.naming.Resolver
type resolver struct {
	scheme      string //前缀
	serviceName string //service name to resolve
}

// NewResolver return resolver with service name
func NewResolver(scheme string, serviceName string) *resolver {
	return &resolver{scheme: scheme, serviceName: serviceName}
}

// Resolve to resolve the service from etcd, target is the dial address of etcd
// target example: "http://127.0.0.1:2379;http://127.0.0.1:12379;http://127.0.0.1:22379"
func (re *resolver) Resolve(etcdAddrs string) (naming.Watcher, error) {
	if re.serviceName == "" {
		return nil, fmt.Errorf("grpc no service name provided")
	}

	if len(etcdAddrs) <= 0 {
		etcdAddrs = os.Getenv(EV_ETCD_ADDR)
	}

	ca := os.Getenv(EV_ETCD_CA_FILE)
	cert := os.Getenv(EV_ETCD_CERT_FILE)
	key := os.Getenv(EV_ETCD_KEY_FILE)

	log.Printf("invoke Resolve: </%v/%v/> from target:%v  etcd_ca_file:%v\n", re.scheme, re.serviceName, etcdAddrs, ca)

	var client *clientv3.Client
	var err error
	if len(ca) > 0 && len(cert) > 0 && len(key) > 0 {
		tlsInfo := transport.TLSInfo{
			CertFile:      cert,
			KeyFile:       key,
			TrustedCAFile: ca,
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			log.Fatal(err)
		}
		client, err = clientv3.New(clientv3.Config{
			Endpoints:   strings.Split(etcdAddrs, ";"),
			DialTimeout: 5 * time.Second,
			TLS:         tlsConfig,
		})

	} else {
		// generate etcd client
		client, err = clientv3.New(clientv3.Config{
			Endpoints:   strings.Split(etcdAddrs, ";"),
			DialTimeout: 5 * time.Second,
		})
	}

	if err != nil {
		log.Fatalf("create etcd3 client failed:%v\n", err)
		return nil, fmt.Errorf("create etcd3 client failed: %v", err)
	}

	log.Printf("before create watcher\n")
	// Return watcher
	return &watcher{re: re, client: client}, nil
}

// watcher is the implementaion of grpc.naming.Watcher
type watcher struct {
	re            *resolver // re: Etcd Resolver
	client        *clientv3.Client
	isInitialized bool
}

// Close do nothing
func (w *watcher) Close() {
	w.client.Close()
	log.Printf("invoke watcher close\n")
}

// Next to return the updates
func (w *watcher) Next() ([]*naming.Update, error) {
	// prefix is the etcd prefix/value to watch
	prefix := fmt.Sprintf("/%v/%v/", w.re.scheme, w.re.serviceName)

	log.Printf("watcher:%v\n", prefix)

	// check if is initialized
	if !w.isInitialized {
		// query addresses from etcd
		resp, err := w.client.Get(context.Background(), prefix, clientv3.WithPrefix())
		log.Printf("watcher rsp:%v\n", resp)
		w.isInitialized = true
		if err == nil {
			addrs := extractAddrs(resp)
			log.Printf("watcher addrs:%v\n", addrs)
			//if not empty, return the updates or watcher new dir
			if l := len(addrs); l != 0 {
				updates := make([]*naming.Update, l)
				for i := range addrs {
					updates[i] = &naming.Update{Op: naming.Add, Addr: addrs[i]}
				}
				return updates, nil
			}
		}
	}

	// generate etcd Watcher
	rch := w.client.Watch(context.Background(), prefix, clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch ev.Type {
			case mvccpb.PUT:
				return []*naming.Update{{Op: naming.Add, Addr: string(ev.Kv.Value)}}, nil
			case mvccpb.DELETE:
				return []*naming.Update{{Op: naming.Delete, Addr: string(ev.Kv.Value)}}, nil
			}
		}
	}
	return nil, nil
}

func extractAddrs(resp *clientv3.GetResponse) []string {
	addrs := []string{}
	if resp == nil || resp.Kvs == nil {
		return addrs
	}

	for i := range resp.Kvs {
		if v := resp.Kvs[i].Value; v != nil {
			addrs = append(addrs, string(v))
		}
	}
	return addrs
}
