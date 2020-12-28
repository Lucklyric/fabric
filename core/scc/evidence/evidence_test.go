/*
Copyright Hitachi America Ltd. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package evidence 

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-chaincode-go/shimtest"
)

func checkInit(t *testing.T, stub *shimtest.MockStub, args [][]byte) {
	res := stub.MockInit("1", args)
	if res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}
}

func checkState(t *testing.T, stub *shimtest.MockStub, name string, value string) {
	bytes := stub.State[name]
	if bytes == nil {
		fmt.Println("State", name, "failed to get value")
		t.FailNow()
	}
	if string(bytes) != value {
		fmt.Println("State value", name, "was not", value, "as expected")
		t.FailNow()
	}
}

func checkQuery(t *testing.T, stub *shimtest.MockStub, name string, value string) {
	res := stub.MockInvoke("1", [][]byte{[]byte("query"), []byte(name)})
	if res.Status != shim.OK {
		fmt.Println("Query", name, "failed", string(res.Message))
		t.FailNow()
	}
	if res.Payload == nil {
		fmt.Println("Query", name, "failed to get value")
		t.FailNow()
	}
	if string(res.Payload) != value {
		fmt.Println("Query value", name, "was not", value, "as expected")
		t.FailNow()
	}
}

func checkInvoke(t *testing.T, stub *shimtest.MockStub, args [][]byte)(string) {
	res := stub.MockInvoke("1", args)
	if res.Status != shim.OK {
		fmt.Println("Invoke", args, "failed", string(res.Message))
		t.FailNow()
	}
    return string(res.Payload)
}

func TestSacc_Query(t *testing.T) {
	cc := new(Evidence)
	stub := shimtest.NewMockStub("evidence", cc)

	// Invoke  set
    hash:= checkInvoke(t, stub, [][]byte{[]byte("put"), []byte("{\"a\":1,\"b\":\"b\"}")})

	// Query by hash 
    checkQuery(t, stub, hash, "{\"a\":1,\"b\":\"b\"}")
}


func TestSacc_InitWithIncorrectArguments(t *testing.T) {
	cc := new(Evidence)
	stub := shimtest.NewMockStub("evidence", cc)

    // Init with incorrect arguments
    res := stub.MockInit("1", [][]byte{[]byte("a"), []byte("10"), []byte("10")})

    if res.Status != shim.ERROR {
        fmt.Println("Invalid Init accepted")
        t.FailNow()
    }

    if res.Message != "Incorrect arguments. Not expecting any init arguments" {
        fmt.Println("Unexpected Error message:", string(res.Message))
        t.FailNow()
    }
}

func TestSacc_QueryWithIncorrectArguments(t *testing.T) {
    cc := new(Evidence)
    stub := shimtest.NewMockStub("evidence", cc)

	// Invoke  set
    checkInvoke(t, stub, [][]byte{[]byte("put"), []byte("{\"a\":1,\"b\":\"b\"}")})

    // Query with incorrect arguments
    res := stub.MockInvoke("1", [][]byte{[]byte("get"), []byte("a"), []byte("b")})

    if res.Status != shim.ERROR {
        fmt.Println("Invalid query accepted")
        t.FailNow()
    }

    if res.Message != "Incorrect arguments. Expecting an evidence hash key" {
        fmt.Println("Unexpected Error message:", string(res.Message))
        t.FailNow()
    }
}


func TestSacc_QueryForAssetNotFound(t *testing.T) {
    cc := new(Evidence)
    stub := shimtest.NewMockStub("evidence", cc)


    // Query for b (as a asset not found)
    res := stub.MockInvoke("1", [][]byte{[]byte("get"), []byte("b")})

    if res.Status != shim.ERROR {
        fmt.Println("Invalid query accepted")
        t.FailNow()
    }

    if res.Message != "Asset not found: b" {
        fmt.Println("Unexpected Error message:", string(res.Message))
        t.FailNow()
    }
}

func TestSacc_InvokeWithIncorrectArguments(t *testing.T) {
    cc := new(Evidence)
    stub := shimtest.NewMockStub("evidence", cc)

    // Invoke with incorrect arguments
    res := stub.MockInvoke("1", [][]byte{[]byte("set"), []byte("a"),[]byte("a")})
    if res.Status != shim.ERROR {
        fmt.Println("Invalid Invoke accepted")
        t.FailNow()
    }

    if res.Message != "Incorrect arguments. Expecting a value only" {
        fmt.Println("Unexpected Error message:", string(res.Message))
        t.FailNow()
    }
}
