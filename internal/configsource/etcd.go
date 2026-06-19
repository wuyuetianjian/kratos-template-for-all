package configsource

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdSource struct {
	client *clientv3.Client
	key    string
}

func NewEtcdSource(client *clientv3.Client, key string) config.Source {
	return &EtcdSource{client: client, key: key}
}

func (s *EtcdSource) Load() ([]*config.KeyValue, error) {
	resp, err := s.client.Get(context.Background(), s.key)
	if err != nil {
		return nil, err
	}
	kvs := make([]*config.KeyValue, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		kvs = append(kvs, s.keyValue(string(kv.Key), kv.Value))
	}
	return kvs, nil
}

func (s *EtcdSource) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &etcdWatcher{
		source: s,
		cancel: cancel,
		ch:     s.client.Watch(ctx, s.key),
	}, nil
}

func (s *EtcdSource) keyValue(key string, value []byte) *config.KeyValue {
	return &config.KeyValue{
		Key:    filepath.Base(key),
		Value:  value,
		Format: strings.TrimPrefix(filepath.Ext(key), "."),
	}
}

type etcdWatcher struct {
	source *EtcdSource
	cancel context.CancelFunc
	ch     clientv3.WatchChan
}

func (w *etcdWatcher) Next() ([]*config.KeyValue, error) {
	resp, ok := <-w.ch
	if !ok {
		return nil, context.Canceled
	}
	if err := resp.Err(); err != nil {
		return nil, err
	}
	kvs := make([]*config.KeyValue, 0, len(resp.Events))
	for _, event := range resp.Events {
		if event.Kv == nil {
			continue
		}
		kvs = append(kvs, w.source.keyValue(string(event.Kv.Key), event.Kv.Value))
	}
	return kvs, nil
}

func (w *etcdWatcher) Stop() error {
	w.cancel()
	return nil
}
