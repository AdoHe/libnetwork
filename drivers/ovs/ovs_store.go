package ovs

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/netlabel"
)

const ovsPrefix = "ovs"

func (d *driver) initStore(option map[string]interface{}) error {
	var err error

	provider, provOK := option[netlabel.LocalKVProvider]
	provURL, urlOK := option[netlabel.LocalKVProviderURL]

	if provOK && urlOK {
		cfg := &datastore.ScopeCfg{
			Client: datastore.ScopeClientCfg{
				Provider: provider.(string),
				Address:  provURL.(string),
			},
		}

		provConfig, confOK := option[netlabel.LocalKVProviderConfig]
		if confOK {
			cfg.Client.Config = provConfig.(*store.Config)
		}

		d.store, err = datastore.NewDataStore(datastore.LocalScope, cfg)
		if err != nil {
			return fmt.Errorf("ovs driver failed to initialize data store: %v", err)
		}

		return d.populateNetworks()
	}

	return nil
}

func (d *driver) populateNetworks() error {
	kvol, err := d.store.List(datastore.Key(ovsPrefix), &networkConfiguration{})
	if err != nil && err != datastore.ErrKeyNotFound {
		return fmt.Errorf("failed to get ovs network configuration from store: %v", err)
	}

	// It's normal for network configuration state to be empty. Just return
	if err == datastore.ErrKeyNotFound {
		return nil
	}

	for _, kvo := range kvol {
		ncfg := kvo.(*networkConfiguration)
		if err = d.createNetwork(ncfg); err != nil {
			logrus.Warnf("could not create ovs network for id %s name %s while booting up from persistent state", ncfg.ID, ncfg.BridgeName)
		}
	}

	return nil
}

func (d *driver) storeUpdate(kvObject datastore.KVObject) error {
	if d.store == nil {
		logrus.Warnf("ovs data store not initialized. kv object %s is not add to store", datastore.Key(kvObject.Key()...))
		return nil
	}

	if err := d.store.PutObjectAtomic(kvObject); err != nil {
		return fmt.Errorf("failed to update ovs data store for object type %T: %v", kvObject, err)
	}

	return nil
}

func (d *driver) storeDelete(kvObject datastore.KVObject) error {
	if d.store == nil {
		logrus.Debugf("ovs data store not initialized. kv object %s is not deleted from store", datastore.Key(kvObject.Key()...))
		return nil
	}

retry:
	if err := d.store.DeleteObjectAtomic(kvObject); err != nil {
		if err == datastore.ErrKeyModified {
			if err := d.store.GetObject(datastore.Key(kvObject.Key()...), kvObject); err != nil {
				return fmt.Errorf("could not update the kvObject to latest when trying to delete: %v", err)
			}
			goto retry
		}
		return err
	}

	return nil
}

func (ncfg *networkConfiguration) Key() []string {
	return []string{ovsPrefix, ncfg.ID}
}

func (ncfg *networkConfiguration) KeyPrefix() []string {
	return []string{ovsPrefix}
}

func (ncfg *networkConfiguration) Value() []byte {
	b, err := json.Marshal(ncfg)
	if err != nil {
		return nil
	}
	return b
}

func (ncfg *networkConfiguration) SetValue(value []byte) error {
	return json.Unmarshal(value, ncfg)
}

func (ncfg *networkConfiguration) Index() uint64 {
	return ncfg.dbIndex
}

func (ncfg *networkConfiguration) SetIndex(index uint64) {
	ncfg.dbIndex = index
	ncfg.dbExists = true
}

func (ncfg *networkConfiguration) Exists() bool {
	return ncfg.dbExists
}

func (ncfg *networkConfiguration) Skip() bool {
	return ncfg.DefaultBridge
}

func (ncfg *networkConfiguration) New() datastore.KVObject {
	return &networkConfiguration{}
}

func (ncfg *networkConfiguration) CopyTo(o datastore.KVObject) error {
	dstNcfg := o.(*networkConfiguration)
	*dstNcfg = *ncfg
	return nil
}

func (ncfg *networkConfiguration) DataScope() string {
	return datastore.LocalScope
}
