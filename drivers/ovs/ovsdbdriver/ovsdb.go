package ovsdbdriver

import (
	"fmt"
	"os/exec"
	"reflect"
	"time"

	"github.com/AdoHe/libovsdb"
	"github.com/docker/libnetwork/drivers/ovs/util"
	"github.com/docker/libnetwork/netutils"
)

const (
	OvsDatabase    = "Open_vSwitch"
	RootTable      = "Open_vSwitch"
	BridgeTable    = "Bridge"
	PortTable      = "Port"
	InterfaceTable = "Interface"

	InsertOp = "insert"
	DeleteOp = "delete"
	SelectOp = "select"
	MutateOp = "mutate"
)

var (
	ovsdbCache   map[string]map[libovsdb.UUID]libovsdb.Row
	contextCache map[string]string
)

type OvsdbDriver struct {
	client *libovsdb.OvsdbClient
}

func NewOvsdber(host string, port int) (*OvsdbDriver, error) {
	var client *libovsdb.OvsdbClient
	var err error
	retries := 3
	for i := 0; i < retries; i++ {
		client, err = libovsdb.ConnectUnix("")
		if err == nil {
			break
		}
	}

	if client == nil {
		return nil, &ErrInvalidOvsDBConnection{}
	}

	ovs := &OvsdbDriver{client: client}
	// Initialize ovsdb cache at rpc connection setup
	ovs.initDBCache()
	return ovs, nil
}

func (ovsdber *OvsdbDriver) initDBCache() {
	ovsdbCache = make(map[string]map[libovsdb.UUID]libovsdb.Row)

	// Register for ovsdb table notifications
	var notifier Notifier
	ovsdber.client.Register(notifier)
	// Populate ovsdb cache for the default Open_vSwitch db
	initCache, err := ovsdber.client.MonitorAll(OvsDatabase, "")
	if err != nil {
		fmt.Printf("Err populating initial OVSDB cache: %v", err)
	}
	populateCache(*initCache)
	contextCache = make(map[string]string)
	populateContextCache(ovsdber.client)

	for ovsdber.getRootUUID().GoUuid == "" {
		time.Sleep(1 * time.Second)
	}
}

func (ovsdber *OvsdbDriver) getRootUUID() libovsdb.UUID {
	for uuid := range ovsdbCache[OvsDatabase] {
		return uuid
	}
	return libovsdb.UUID{}
}

// Check if bridge exists prior to create a bridge
func (ovsdber *OvsdbDriver) AddOvsBridge(bridgeName string, setMacAddr bool) error {
	if ovsdber.client == nil {
		return &ErrInvalidOvsDBConnection{}
	}

	// If the bridge has been created, an internal port with the same name will exist
	exists, err := ovsdber.bridgeExists(bridgeName)
	if err != nil {
		fmt.Printf("check bridge exists err: %v\n", err)
		return err
	}
	if !exists {
		if err := ovsdber.createOvsBridge(bridgeName, setMacAddr); err != nil {
			fmt.Printf("create ovs bridge error: %v\n", err)
			return err
		}
		exists, err = ovsdber.bridgeExists(bridgeName)
		if err != nil {
			fmt.Printf("check bridge exists err: %v\n", err)
			return err
		}
		if !exists {
			return fmt.Errorf("Error creating ovs bridge")
		}
	} else {
		return ErrBridgeAlreadyExists(bridgeName)
	}

	return nil
}

// Check if bridge exists prior to delete a bridge
func (ovsdber *OvsdbDriver) RemoveOvsBridge(bridgeName string) error {
	if ovsdber.client == nil {
		return &ErrInvalidOvsDBConnection{}
	}

	exists, err := ovsdber.bridgeExists(bridgeName)
	if err != nil {
		return err
	}
	if exists {
		if err := ovsdber.deleteOvsBridge(bridgeName); err != nil {
			return err
		}
		exists, err = ovsdber.bridgeExists(bridgeName)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("Error deleting ovs bridge")
		}
	} else {
		return ErrBridgeNotExists(bridgeName)
	}
	return nil
}

// CreateOvsBridge creates an OVS bridge.
func (ovsdber *OvsdbDriver) createOvsBridge(bridgeName string, setMacAddr bool) error {
	namedBridgeUuid := "bridge"
	namedPortUuid := "port"
	namedIntfUuid := "intf"
	protocols := []string{"OpenFlow10", "OpenFlow11", "OpenFlow12", "OpenFlow13"}

	// intf row to insert
	intf := make(map[string]interface{})
	intf["name"] = bridgeName
	intf["type"] = `internal`

	intfOp := libovsdb.Operation{
		Op:       InsertOp,
		Table:    InterfaceTable,
		Row:      intf,
		UUIDName: namedIntfUuid,
	}

	// port row to insert
	port := make(map[string]interface{})
	port["name"] = bridgeName
	port["interfaces"] = libovsdb.UUID{namedIntfUuid}

	portOp := libovsdb.Operation{
		Op:       InsertOp,
		Table:    PortTable,
		Row:      port,
		UUIDName: namedPortUuid,
	}

	// bridge row to insert
	bridge := make(map[string]interface{})
	bridge["name"] = bridgeName
	bridge["stp_enable"] = false
	bridge["protocols"], _ = libovsdb.NewOvsSet(protocols)
	bridge["ports"] = libovsdb.UUID{namedPortUuid}
	if setMacAddr {
		bridge["other_config:hwaddr"] = netutils.GenerateRandomMAC().String()
	}

	bridgeOp := libovsdb.Operation{
		Op:       InsertOp,
		Table:    BridgeTable,
		Row:      bridge,
		UUIDName: namedBridgeUuid,
	}

	// Inserting a Bridge row in Bridge table requires mutating the open_vswitch table.
	mutateUuid := []libovsdb.UUID{libovsdb.UUID{namedBridgeUuid}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUuid)
	mutation := libovsdb.NewMutation("bridges", InsertOp, mutateSet)
	condition := libovsdb.NewCondition("_uuid", "==", ovsdber.getRootUUID())

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        MutateOp,
		Table:     RootTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{intfOp, portOp, bridgeOp, mutateOp}
	return ovsdber.performOvsdbOps(operations)
}

// DeleteOvsBridge deletes an OVS bridge.
func (ovsdber *OvsdbDriver) deleteOvsBridge(bridgeName string) error {
	/*namedBridgeUUID := "bridge"
	brUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: namedBridgeUUID}}

	// simple delete operation.
	condition := libovsdb.NewCondition("name", "==", bridgeName)
	bridgeOp := libovsdb.Operation{
		Op:    DeleteOp,
		Table: BridgeTable,
		Where: []interface{}{condition},
	}

	// also fetch the br-uuid from cache
	for uuid, row := range getTableCache(BridgeTable) {
		name := row.Fields["name"].(string)
		if name == bridgeName {
			brUUID = []libovsdb.UUID{uuid}
			break
		}
	}

	// Delete a Bridge row in Bridge table requires mutating the Open_vSwitch table.
	mutateUUID := brUUID
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("bridges", "delete", mutateSet)
	condition = libovsdb.NewCondition("_uuid", "==", ovsdber.getRootUUID())

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        MutateOp,
		Table:     RootTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{bridgeOp, mutateOp}
	return ovsdber.performOvsdbOps(operations)*/
	return ovsctlDeleteBridge(bridgeName)
}

// bridgeExists checks whether the bridge exists
func (ovsdber *OvsdbDriver) bridgeExists(bridgeName string) (bool, error) {
	condition := libovsdb.NewCondition("name", "==", bridgeName)
	selectOp := libovsdb.Operation{
		Op:    SelectOp,
		Table: BridgeTable,
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{selectOp}
	reply, _ := ovsdber.client.Transact(OvsDatabase, operations...)

	if len(reply) < len(operations) {
		return false, &ErrReplyDisMatchOps{}
	}

	if reply[0].Error != "" {
		return false, &ErrTransactionError{errMsg: reply[0].Error}
	}

	if len(reply[0].Rows) == 0 {
		return false, nil
	}
	return true, nil
}

func (ovsdber *OvsdbDriver) performOvsdbOps(ops []libovsdb.Operation) error {
	if len(ops) == 0 {
		return nil
	}

	reply, _ := ovsdber.client.Transact(OvsDatabase, ops...)

	if len(reply) < len(ops) {
		return &ErrReplyDisMatchOps{}
	}

	errs := []error(nil)
	for _, o := range reply {
		if o.Error != "" {
			errs = append(errs, fmt.Errorf("%s(%s)", o.Error, o.Details))
		}
	}

	return util.NewAggregate(errs)
}

func (ovsdber *OvsdbDriver) Disconnect() {
	ovsdber.client.Disconnect()
}

func (ovsdber *OvsdbDriver) Reconnect() {
}

type Notifier struct {
}

func (n Notifier) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	populateCache(tableUpdates)
}

func (n Notifier) Locked([]interface{}) {
}

func (n Notifier) Stolen([]interface{}) {
}

func (n Notifier) Echo([]interface{}) {
}
func (n Notifier) Disconnected(client *libovsdb.OvsdbClient) {
}

func populateCache(updates libovsdb.TableUpdates) {
	for table, tableUpdate := range updates.Updates {
		if _, ok := ovsdbCache[table]; !ok {
			ovsdbCache[table] = make(map[libovsdb.UUID]libovsdb.Row)
		}
		for uuid, row := range tableUpdate.Rows {
			empty := libovsdb.Row{}
			if !reflect.DeepEqual(row.New, empty) {
				ovsdbCache[table][libovsdb.UUID{GoUuid: uuid}] = row.New
			} else {
				delete(ovsdbCache[table], libovsdb.UUID{GoUuid: uuid})
			}
		}
	}
}

func populateContextCache(client *libovsdb.OvsdbClient) {
	if client == nil {
		return
	}

	tableCache := getTableCache("Interface")
	for _, row := range tableCache {
		config, ok := row.Fields["other_config"]
		ovsMap := config.(libovsdb.OvsMap)
		otherConfig := map[interface{}]interface{}(ovsMap.GoMap)
		if ok {
			containerID, ok := otherConfig[""]
			if ok {
				contextCache[containerID.(string)] = otherConfig[""].(string)
			}
		}
	}
}

func getTableCache(tableName string) map[libovsdb.UUID]libovsdb.Row {
	return ovsdbCache[tableName]
}

// helper method for test convenience
func ovsctlCreateBridge(bridgeName string) error {
	if _, err := exec.Command("ovs-vsctl", "add-br", bridgeName).Output(); err != nil {
		return fmt.Errorf("ovs-vsct failed to create ovs bridge %s: %v", bridgeName, err)
	}
	return nil
}

// helper method fo test convenience
func ovsctlDeleteBridge(bridgeName string) error {
	if _, err := exec.Command("ovs-vsctl", "del-br", bridgeName).Output(); err != nil {
		return fmt.Errorf("ovs-vsctl failed to delete ovs bridge %s: %v", bridgeName, err)
	}
	return nil
}
