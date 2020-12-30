/*
 * Copyright IBM Corp All Rights Reserved
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/flogging"
)

type Evidence struct {
}

func New() *Evidence {
	return &Evidence{}
}

func (e *Evidence) Name() string              { return "evidence" }
func (e *Evidence) Chaincode() shim.Chaincode { return e }

var escclogger = flogging.MustGetLogger("evidence")

func (t *Evidence) Init(stub shim.ChaincodeStubInterface) peer.Response {
	// Get the args from the transaction proposal
	args := stub.GetStringArgs()
	escclogger.Info("init ESCC")

	if len(args) != 0 {
		return shim.Error("Incorrect arguments. Not expecting any init arguments")
	}

	return shim.Success(nil)
}

func (t *Evidence) Invoke(stub shim.ChaincodeStubInterface) peer.Response {
	// Extract the function and args from the transaction proposal
	fn, args := stub.GetFunctionAndParameters()

	var result string
	var err error
	if fn == "put" {
		result, err = put(stub, args)
	} else { // assume 'get' even if fn is nil
		result, err = get(stub, args)
	}
	if err != nil {
		return shim.Error(err.Error())
	}

	// Return the result as success payload
	return shim.Success([]byte(result))
}

func put(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a value only")
	}
	escclogger.Infof("New evidence for channel: %s", stub.GetChannelID())
	hash := sha256.Sum256([]byte(args[0] + stub.GetChannelID()))
	hashString := hex.EncodeToString(hash[:])
	errPut := stub.PutState(hashString, []byte(args[0]))
	if errPut != nil {
		return "", fmt.Errorf("Failed to put evidence: %s", args[0])
	}
	return hashString, nil
}

// Get returns the value of the specified asset key
func get(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("Incorrect arguments. Expecting an evidence hash key")
	}

	value, err := stub.GetState(args[0])
	if err != nil {
		return "", fmt.Errorf("Failed to get evidence: %s with error: %s", args[0], err)
	}
	if value == nil {
		return "", fmt.Errorf("Evidence not found: %s", args[0])
	}
	return string(value), nil
}

// main function starts up the chaincode in the container during instantiate
func main() {
	if err := shim.Start(new(Evidence)); err != nil {
		fmt.Printf("Error starting Evidence chaincode: %s", err)
	}
}
