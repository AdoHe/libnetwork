package ovsdbdriver

import (
	"fmt"
	"os/exec"

	"github.com/AdoHe/libovsdb"
)

// add ovs internal port
func (ovsdber *OvsdbDriver) AddOvsInternalPort(bridgeName, portName string, tag uint) error {
	return ovsdber.addPort(bridgeName, portName, "internal", tag)
}

// add veth pair port
func (ovsdber *OvsdbDriver) AddOvsVethPort(bridgeName, portName string, tag uint) error {
	return ovsdber.addPort(bridgeName, portName, "", tag)
}

func (ovsdber *OvsdbDriver) addPort(bridgeName, portName, portType string, tag uint) error {
	namedPortUUIDStr := portName
	namedIntfUUIDStr := fmt.Sprintf("Intf%s", portName)
	namedPortUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: namedPortUUIDStr}}
	namedIntfUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: namedIntfUUIDStr}}

	var err error

	// intf row to insert
	intf := make(map[string]interface{})
	intf["name"] = portName
	intf["type"] = portType

	intfOp := libovsdb.Operation{
		Op:       InsertOp,
		Table:    InterfaceTable,
		Row:      intf,
		UUIDName: namedIntfUUIDStr,
	}

	// port row to insert
	port := make(map[string]interface{})
	port["name"] = portName

	if tag != 0 {
		port["vlan_mode"] = "access"
		port["tag"] = tag
	}
	port["interfaces"], err = libovsdb.NewOvsSet(namedIntfUUID)
	if err != nil {
		return fmt.Errorf("failed to create port interfaces set")
	}

	portOp := libovsdb.Operation{
		Op:       InsertOp,
		Table:    PortTable,
		Row:      port,
		UUIDName: namedPortUUIDStr,
	}

	// inserting a row in Port table requires mutating the bridge table.
	mutateSet, _ := libovsdb.NewOvsSet(namedPortUUID)
	mutation := libovsdb.NewMutation("ports", InsertOp, mutateSet)
	condition := libovsdb.NewCondition("name", "==", bridgeName)

	mutateOp := libovsdb.Operation{
		Op:        MutateOp,
		Table:     BridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return ovsdber.performOvsdbOps(operations)
}

func (ovsdber *OvsdbDriver) DeletePort(bridgeName, portName string) error {
	portUUIDStr := portName
	portUUID := []libovsdb.UUID{{portUUIDStr}}

	condition := libovsdb.NewCondition("name", "==", portName)
	intfOp := libovsdb.Operation{
		Op:    DeleteOp,
		Table: InterfaceTable,
		Where: []interface{}{condition},
	}

	condition = libovsdb.NewCondition("name", "==", portName)
	portOp := libovsdb.Operation{
		Op:    DeleteOp,
		Table: PortTable,
		Where: []interface{}{condition},
	}

	for uuid, row := range getTableCache(PortTable) {
		name := row.Fields["name"].(string)
		if name == portName {
			portUUID = []libovsdb.UUID{uuid}
			break
		}
	}

	// Deleting a port row in Port table requires mutating the Bridge table.
	mutateSet, _ := libovsdb.NewOvsSet(portUUID)
	mutation := libovsdb.NewMutation("ports", DeleteOp, mutateSet)
	condition = libovsdb.NewCondition("name", "==", bridgeName)

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        MutateOp,
		Table:     BridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return ovsdber.performOvsdbOps(operations)
}

func (ovsdber *OvsdbDriver) AddVxLanPort(bridgeName, portName string, peerAddr string) error {
	namedPortUUID := "port"
	namedIntfUUID := "intf"

	options := make(map[string]interface{})
	options["remote_ip"] = peerAddr

	// intf row to insert
	intf := make(map[string]interface{})
	intf["name"] = portName
	intf["type"] = `vxlan`
	intf["options"], _ = libovsdb.NewOvsMap(options)

	intfOp := libovsdb.Operation{
		Op:       InsertOp,
		Table:    InterfaceTable,
		Row:      intf,
		UUIDName: namedIntfUUID,
	}

	// port row to insert
	port := make(map[string]interface{})
	port["name"] = portName
	port["interfaces"] = libovsdb.UUID{namedIntfUUID}

	portOp := libovsdb.Operation{
		Op:       InsertOp,
		Table:    PortTable,
		Row:      port,
		UUIDName: namedPortUUID,
	}

	// Inserting a row in Port table requires mutating the Bridge table.
	mutateUUID := []libovsdb.UUID{libovsdb.UUID{namedPortUUID}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("ports", InsertOp, mutateSet)
	mutateCond := libovsdb.NewCondition("name", "==", bridgeName)

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        MutateOp,
		Table:     BridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{mutateCond},
	}

	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return ovsdber.performOvsdbOps(operations)
}

// portExists checks whether the port exists
func (ovsdber *OvsdbDriver) portExists(portName string) (bool, error) {
	condition := libovsdb.NewCondition("name", "==", portName)
	selectOp := libovsdb.Operation{
		Op:    SelectOp,
		Table: PortTable,
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

// helper method for test convenience
func OvsctlCreateNormalPort(bridgeName, portName string) error {
	if _, err := exec.Command("ovs-vsctl", "add-port", bridgeName, portName).Output(); err != nil {
		return fmt.Errorf("ovs-vsctl failed to create port %s on bridge %s: %v", portName, bridgeName, err)
	}
	return nil
}

// helper method for test convenience
func OvsctlCreateInternalPort(bridgeName, portName string) error {
	if _, err := exec.Command("ovs-vsctl", "add-port", bridgeName, portName, "--", "set", "Inteface", portName, "type=internal").Output(); err != nil {
		return fmt.Errorf("ovs-vsctl failed to create internal port %s on bridge %s: %v", portName, bridgeName, err)
	}
	return nil
}

// helper method for test convenience
func OvsctlDeletePort(bridgeName, portName string) error {
	if _, err := exec.Command("ovs-vsctl", "--if-exists", "del-port", bridgeName, portName).Output(); err != nil {
		return fmt.Errorf("ovs-vsctl failed to delete port %s on bridge %s: %v", portName, bridgeName, err)
	}
	return nil
}
