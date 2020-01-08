package api

import (
	"context"
	"log"
	"strings"
	"time"
	"fmt"

	//"github.com/coreos/etcd/clientv3"
	"go.etcd.io/etcd/clientv3"
)


type etcd3Register struct {
	ctx  context.Context
	cli  *clientv3.Client
	key    string
	scheme string
	name   string
	addr   string
	ttl    int64
	status  int32
	stop    chan bool
}

var register *etcd3Register = nil

func Register(etcdAddr string, scheme string, serviceName string, serviceAddr string, ttl int64) error {
	if register != nil {
		return fmt.Errorf("service<%v> has been registered",serviceName)
	}
	register = &etcd3Register{stop: make(chan bool),}
	return register.register(etcdAddr, scheme,serviceName, serviceAddr, ttl)
}

func UnRegister() {
	if register != nil {
		register.unRegister()
	}
}

// Register register service with name as prefix to etcd, multi etcd addr should use ; to split
func (p *etcd3Register) register(etcdAddr string, scheme string, serviceName string, serviceAddr string, ttl int64) error {
	if p.status > 0 {
		return fmt.Errorf("service<%v> has been registered.", serviceName)
	}

	var err error
	if p.cli == nil {
		p.cli, err = clientv3.New(clientv3.Config{
			Endpoints:   strings.Split(etcdAddr, ";"),
			DialTimeout: 20 * time.Second,
		})
		if err != nil {
			return err
		}
	}

	p.ctx = context.Background()
	p.scheme = strings.ToLower(scheme)
	p.name = strings.ToLower(serviceName)
	p.addr = strings.ToLower(serviceAddr)
	p.key = fmt.Sprintf("/%v/%v/%v", 	p.scheme, p.name , p.addr)
	p.ttl = ttl

	go func() {
		for {
			select {
				case  <- p.stop :
					log.Printf("service<%v> stop!\n", p.key)
					return
				case  <-  p.cli.Ctx().Done():
					log.Printf("service<%v> etcd-stop!\n", p.key)
					return
				default:
			}

			rsp, err := p.cli.Get(p.ctx, p.key)
			if err != nil {
				log.Fatalf("service<%v> clientv3 get key fail! error<%v>\n", p.key, err)
			} else if rsp.Count == 0 {
				err = p.keepAlive()
				if err != nil {
					log.Fatalf("service<%v> clientv3 keepAlive key fail!", p.key, err)
				}
			}

			log.Printf("service<%v> etcd-get-key-value: %v", p.key, rsp)
			time.Sleep(time.Second * 20)
		}
	}()

	p.status = 1
	log.Printf("service<%v> register ok.\n", p.key)
	return nil
}

func (p *etcd3Register) keepAlive() error {
	leaseResp, err := p.cli.Grant(p.ctx, p.ttl)
	if err != nil {
		return err
	}

	_, err = p.cli.Put(p.ctx, p.key, p.addr, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return err
	}

	ch, err := p.cli.KeepAlive(p.ctx, leaseResp.ID)
	if err != nil {
		return err
	}

	log.Printf("service<%v> keepAlive lease-id:%v", p.key, leaseResp.ID)

	for {
		select {
		case  <- p.stop :
			return fmt.Errorf("service<%v> stop!\n", p.key)
		case  <-  p.cli.Ctx().Done():
			return fmt.Errorf("service<%v> etcd-stop!\n", p.key)
		case  dd ,ok := <- ch:
			if !ok {
				return fmt.Errorf("service<%v> clientv3 keepAlive channel close", p.key)
			}
			log.Printf("service<%v> keepAlive recv: %v\n",p.key, dd)
		default:
		}
	}

	//解决 {"level":"warn","ts":"2019-11-27T16:27:50.161+0800","caller":"clientv3/lease.go:524","msg":"lease keepalive response queue is full; dropping response send","queue-size":16,"queue-capacity":16}
	//go func(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	//	for {
	//		_ = <-ch
	//		//log.Printf("keepAlive recv: %v\n", dd)
	//	}
	//} (ch)

	return nil
}

// UnRegister remove service from etcd
func (p *etcd3Register) unRegister() {
	if p.cli != nil {
		p.stop <- true
		p.cli.Delete(p.ctx, p.key)
		p.cli.Close()
		close(p.stop)
		log.Printf("service<%v> unregister ok.\n", p.key)
	}
	p.status = 0
}



