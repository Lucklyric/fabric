/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package plain_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/ledger/customtx"
	"github.com/hyperledger/fabric/protos/token"
	mockid "github.com/hyperledger/fabric/token/identity/mock"
	mockledger "github.com/hyperledger/fabric/token/ledger/mock"
	"github.com/hyperledger/fabric/token/tms/plain"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const tokenNamespace = "_fabtoken"

var _ = Describe("Verifier", func() {
	var (
		fakePublicInfo       *mockid.PublicInfo
		fakeIssuingValidator *mockid.IssuingValidator
		fakeLedger           *mockledger.LedgerWriter
		memoryLedger         *plain.MemoryLedger

		importTransaction *token.TokenTransaction
		importTxID        string

		verifier *plain.Verifier
	)

	BeforeEach(func() {
		fakePublicInfo = &mockid.PublicInfo{}
		fakeIssuingValidator = &mockid.IssuingValidator{}
		fakeLedger = &mockledger.LedgerWriter{}
		fakeLedger.SetStateReturns(nil)

		importTxID = "0"
		importTransaction = &token.TokenTransaction{
			Action: &token.TokenTransaction_PlainAction{
				PlainAction: &token.PlainTokenAction{
					Data: &token.PlainTokenAction_PlainImport{
						PlainImport: &token.PlainImport{
							Outputs: []*token.PlainOutput{
								{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 111},
								{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK2", Quantity: 222},
							},
						},
					},
				},
			},
		}

		verifier = &plain.Verifier{
			IssuingValidator: fakeIssuingValidator,
		}
	})

	Describe("ProcessTx PlainImport", func() {
		It("evaluates policy for each output", func() {
			err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeIssuingValidator.ValidateCallCount()).To(Equal(2))
			creator, tt := fakeIssuingValidator.ValidateArgsForCall(0)
			Expect(creator).To(Equal(fakePublicInfo))
			Expect(tt).To(Equal("TOK1"))
			creator, tt = fakeIssuingValidator.ValidateArgsForCall(1)
			Expect(creator).To(Equal(fakePublicInfo))
			Expect(tt).To(Equal("TOK2"))
		})

		It("checks the fake ledger", func() {
			err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLedger.SetStateCallCount()).To(Equal(2))

			outputBytes, err := proto.Marshal(&token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 111})
			Expect(err).NotTo(HaveOccurred())
			ns, k, td := fakeLedger.SetStateArgsForCall(0)
			Expect(ns).To(Equal(tokenNamespace))
			expectedOutput := strings.Join([]string{"", "tokenOutput", "0", "0", ""}, "\x00")
			Expect(k).To(Equal(expectedOutput))
			Expect(td).To(Equal(outputBytes))

			outputBytes, err = proto.Marshal(&token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK2", Quantity: 222})
			Expect(err).NotTo(HaveOccurred())
			ns, k, td = fakeLedger.SetStateArgsForCall(1)
			Expect(ns).To(Equal(tokenNamespace))
			expectedOutput = strings.Join([]string{"", "tokenOutput", "0", "1", ""}, "\x00")
			Expect(k).To(Equal(expectedOutput))
			Expect(td).To(Equal(outputBytes))
		})

		Context("when policy validation fails", func() {
			BeforeEach(func() {
				fakeIssuingValidator.ValidateReturns(errors.New("no-way-man"))
			})

			It("returns an error and does not write to the ledger", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "import policy check failed: no-way-man"}))
				Expect(fakeLedger.SetStateCallCount()).To(Equal(0))
			})
		})

		Context("when the ledger write check fails", func() {
			BeforeEach(func() {
				fakeLedger.SetStateReturns(errors.New("no-can-do"))
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("no-can-do"))

				Expect(fakeLedger.SetStateCallCount()).To(Equal(1))
			})
		})

		Context("when transaction does not contain any outputs", func() {
			BeforeEach(func() {
				importTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainImport{
								PlainImport: &token.PlainImport{
									Outputs: []*token.PlainOutput{},
								},
							},
						},
					},
				}
			})
			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "no outputs in transaction: 0"}))
			})
		})

		Context("when the output of a transaction has quantity of 0", func() {
			BeforeEach(func() {
				importTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainImport{
								PlainImport: &token.PlainImport{
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 0},
									},
								},
							},
						},
					},
				}
			})
			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "output 0 quantity is 0 in transaction: 0"}))
			})
		})

		Context("when an output already exists", func() {
			BeforeEach(func() {
				memoryLedger = plain.NewMemoryLedger()
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, memoryLedger)
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, memoryLedger)
				Expect(err).To(HaveOccurred())
				existingOutputId := strings.Join([]string{"", "tokenOutput", "0", "0", ""}, "\x00")
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: fmt.Sprintf("output already exists: %s", existingOutputId)}))
			})
		})

		Context("when an output has no owner", func() {
			BeforeEach(func() {
				importTransaction.GetPlainAction().GetPlainImport().Outputs[0].Owner = nil
				importTxID = "no-owner-id"
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: fmt.Sprintf("missing owner in output for txID '%s'", importTxID)}))
			})
		})

	})

	Describe("Output GetState error scenarios", func() {
		BeforeEach(func() {
			memoryLedger = plain.NewMemoryLedger()
			err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, memoryLedger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("retrieves the PlainOutput associated with the entry ID", func() {
			po, err := memoryLedger.GetState(tokenNamespace, strings.Join([]string{"", "tokenOutput", "0", "0", ""}, "\x00"))
			Expect(err).NotTo(HaveOccurred())

			output := &token.PlainOutput{}
			err = proto.Unmarshal(po, output)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(Equal(&token.PlainOutput{
				Owner:    &token.TokenOwner{Raw: []byte("owner-1")},
				Type:     "TOK1",
				Quantity: 111,
			}))

			po, err = memoryLedger.GetState(tokenNamespace, strings.Join([]string{"", "tokenOutput", "0", "1", ""}, "\x00"))
			Expect(err).NotTo(HaveOccurred())

			err = proto.Unmarshal(po, output)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(Equal(&token.PlainOutput{
				Owner:    &token.TokenOwner{Raw: []byte("owner-2")},
				Type:     "TOK2",
				Quantity: 222,
			}))
		})

		Context("when the output does not exist", func() {
			It("returns a nil and no error", func() {
				val, err := memoryLedger.GetState(tokenNamespace, strings.Join([]string{"", "tokenOutput", "george", "0", ""}, "\x00"))
				Expect(err).NotTo(HaveOccurred())
				Expect(val).To(BeNil())
			})
		})
	})

	Describe("ProcessTx empty or invalid input", func() {
		Context("when a plain action is not provided", func() {
			BeforeEach(func() {
				importTxID = "255"
				importTransaction = &token.TokenTransaction{}
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(MatchError("check process failed for transaction '255': missing token action"))
			})
		})

		Context("when an unknown plain token action is provided", func() {
			BeforeEach(func() {
				importTxID = "254"
				importTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{},
					},
				}
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "unknown plain token action: <nil>"}))
			})
		})

		Context("when a transaction has invalid characters in key", func() {
			BeforeEach(func() {
				importTxID = string(0)
			})

			It("fails when creating the ledger key for the output", func() {
				By("returning an error")
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "error creating output ID: input contain unicode U+0000 starting at position [0]. U+0000 and U+10FFFF are not allowed in the input attribute of a composite key"}))
			})
		})

		Context("when a transaction has invalid characters in key", func() {
			BeforeEach(func() {
				importTxID = string(0)
			})

			It("fails when creating the ledger key for the first output", func() {
				By("returning an error")
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "error creating output ID: input contain unicode U+0000 starting at position [0]. U+0000 and U+10FFFF are not allowed in the input attribute of a composite key"}))
			})
		})

		Context("when a transaction key is an invalid utf8 string", func() {
			BeforeEach(func() {
				importTxID = string([]byte{0xE0, 0x80, 0x80})
			})

			It("fails when creating the ledger key for the output", func() {
				By("returning an error")
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "error creating output ID: not a valid utf8 string: [e08080]"}))
			})
		})

		Context("when the ledger read of an output fails", func() {
			BeforeEach(func() {
				fakeLedger.GetStateReturnsOnCall(0, nil, errors.New("error reading output"))
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("error reading output"))

				Expect(fakeLedger.GetStateCallCount()).To(Equal(1))
				Expect(fakeLedger.SetStateCallCount()).To(Equal(0))
				ns, k := fakeLedger.GetStateArgsForCall(0)
				expectedOutput := strings.Join([]string{"", "tokenOutput", "0", "0", ""}, "\x00")
				Expect(k).To(Equal(expectedOutput))
				Expect(ns).To(Equal(tokenNamespace))
			})
		})
	})

	Describe("Test ProcessTx PlainTransfer with memory ledger", func() {
		var (
			transferTransaction *token.TokenTransaction
			transferTxID        string
		)

		BeforeEach(func() {
			transferTxID = "1"
			transferTransaction = &token.TokenTransaction{
				Action: &token.TokenTransaction_PlainAction{
					PlainAction: &token.PlainTokenAction{
						Data: &token.PlainTokenAction_PlainTransfer{
							PlainTransfer: &token.PlainTransfer{
								Inputs: []*token.TokenId{
									{TxId: "0", Index: 0},
								},
								Outputs: []*token.PlainOutput{
									{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 99},
									{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK1", Quantity: 12},
								},
							},
						},
					},
				},
			}
			fakePublicInfo.PublicReturns([]byte("owner-1"))
			memoryLedger = plain.NewMemoryLedger()
			err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, memoryLedger)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when a valid transfer is provided", func() {
			BeforeEach(func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).NotTo(HaveOccurred())
			})

			It("is processed successfully", func() {
				po, err := memoryLedger.GetState(tokenNamespace, string("\x00")+"tokenOutput"+string("\x00")+"1"+string("\x00")+"0"+string("\x00"))
				Expect(err).NotTo(HaveOccurred())

				output := &token.PlainOutput{}
				err = proto.Unmarshal(po, output)
				Expect(err).NotTo(HaveOccurred())

				Expect(output).To(Equal(&token.PlainOutput{
					Owner:    &token.TokenOwner{Raw: []byte("owner-1")},
					Type:     "TOK1",
					Quantity: 99,
				}))

				po, err = memoryLedger.GetState(tokenNamespace, string("\x00")+"tokenOutput"+string("\x00")+"1"+string("\x00")+"1"+string("\x00"))
				Expect(err).NotTo(HaveOccurred())

				err = proto.Unmarshal(po, output)
				Expect(err).NotTo(HaveOccurred())

				Expect(output).To(Equal(&token.PlainOutput{
					Owner:    &token.TokenOwner{Raw: []byte("owner-2")},
					Type:     "TOK1",
					Quantity: 12,
				}))

				spentMarker, err := memoryLedger.GetState(tokenNamespace, string("\x00")+"tokenInput"+string("\x00")+"0"+string("\x00")+"0"+string("\x00"))
				Expect(err).NotTo(HaveOccurred())
				Expect(bytes.Equal(spentMarker, plain.TokenInputSpentMarker)).To(BeTrue())
			})
		})

		Context("when a non-existent input is referenced", func() {
			BeforeEach(func() {
				transferTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer{
								PlainTransfer: &token.PlainTransfer{
									Inputs: []*token.TokenId{
										{TxId: "wild_pineapple", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 99},
										{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK1", Quantity: 12},
									},
								},
							},
						},
					},
				}
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "input with ID \x00tokenOutput\x00wild_pineapple\x000\x00 for transfer does not exist"}))
			})
		})

		Context("when the creator of the transfer transaction is not the owner of the input", func() {
			BeforeEach(func() {
				fakePublicInfo.PublicReturns([]byte("owner-pineapple"))
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "transfer input with ID \x00tokenOutput\x000\x000\x00 not owned by creator"}))
			})
		})

		Context("when the same input is spent twice", func() {
			BeforeEach(func() {
				transferTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer{
								PlainTransfer: &token.PlainTransfer{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 221},
										{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK1", Quantity: 1},
									},
								},
							},
						},
					},
				}
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token input '\x00tokenOutput\x000\x000\x00' spent more than once in transaction ID '1'"}))
			})
		})

		Context("when the input type does not match the output type", func() {
			BeforeEach(func() {
				transferTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer{
								PlainTransfer: &token.PlainTransfer{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "wild_pineapple", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "wild_pineapple", Quantity: 11},
									},
								},
							},
						},
					},
				}
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token type mismatch in inputs and outputs for transaction ID 1 (wild_pineapple vs TOK1)"}))
			})
		})

		Context("when the input sum does not match the output sum", func() {
			BeforeEach(func() {
				transferTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer{
								PlainTransfer: &token.PlainTransfer{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 112},
										{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK1", Quantity: 12},
									},
								},
							},
						},
					},
				}
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token sum mismatch in inputs and outputs for transaction ID 1 (124 vs 111)"}))
			})
		})

		Context("when the input contains multiple token types", func() {
			var (
				anotherImportTransaction *token.TokenTransaction
				anotherImportTxID        string
			)
			BeforeEach(func() {
				anotherImportTxID = "2"
				anotherImportTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainImport{
								PlainImport: &token.PlainImport{
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK2", Quantity: 2121},
									},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(anotherImportTxID, fakePublicInfo, anotherImportTransaction, memoryLedger)
				Expect(err).NotTo(HaveOccurred())
				transferTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer{
								PlainTransfer: &token.PlainTransfer{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
										{TxId: "2", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 111},
									},
								},
							},
						},
					},
				}
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types in input for txID: 1 (TOK1, TOK2)"}))
			})
		})

		Context("when the output contains multiple token types", func() {
			BeforeEach(func() {
				transferTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer{
								PlainTransfer: &token.PlainTransfer{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 112},
										{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK2", Quantity: 12},
									},
								},
							},
						},
					},
				}
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types ('TOK1', 'TOK2') in output for txID '1'"}))
			})
		})

		Context("when an input has already been spent", func() {
			BeforeEach(func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx("2", fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "input with ID \x00tokenOutput\x000\x000\x00 for transfer has already been spent"}))
			})
		})

		Context("when an output already exists", func() {
			BeforeEach(func() {
				transferTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer{
								PlainTransfer: &token.PlainTransfer{
									Inputs: []*token.TokenId{},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "", Quantity: 0},
									},
								},
							},
						},
					},
				}
				memoryLedger = plain.NewMemoryLedger()
				err := verifier.ProcessTx(importTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns an error", func() {
				err := verifier.ProcessTx(importTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(HaveOccurred())
				existingOutputId := string("\x00") + "tokenOutput" + string("\x00") + "0" + string("\x00") + "0" + string("\x00")
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: fmt.Sprintf("output already exists: %s", existingOutputId)}))
			})
		})

		Context("when an output has no owner", func() {
			BeforeEach(func() {
				transferTransaction.GetPlainAction().GetPlainTransfer().Outputs[0].Owner = nil
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx(transferTxID, fakePublicInfo, transferTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: fmt.Sprintf("missing owner in output for txID '%s'", transferTxID)}))
			})
		})
	})

	Describe("Test ProcessTx PlainRedeem with memory ledger", func() {
		var (
			tokenIds          []*token.TokenId
			redeemTxID        string
			redeemTransaction *token.TokenTransaction
		)

		BeforeEach(func() {
			redeemTxID = "r1"
			tokenIds = []*token.TokenId{
				{TxId: "0", Index: 0},
			}
			redeemTransaction = &token.TokenTransaction{
				Action: &token.TokenTransaction_PlainAction{
					PlainAction: &token.PlainTokenAction{
						Data: &token.PlainTokenAction_PlainRedeem{
							PlainRedeem: &token.PlainTransfer{
								Inputs: tokenIds,
								Outputs: []*token.PlainOutput{
									{Type: "TOK1", Quantity: 111},
								},
							},
						},
					},
				},
			}

			fakePublicInfo.PublicReturns([]byte("owner-1"))
			memoryLedger = plain.NewMemoryLedger()
			err := verifier.ProcessTx(importTxID, fakePublicInfo, importTransaction, memoryLedger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("processes a redeem transaction with all tokens redeemed", func() {
			err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
			Expect(err).NotTo(HaveOccurred())

			// verify we can get the output from "tokenRedeem" for this transaction
			po, err := memoryLedger.GetState(tokenNamespace, string("\x00")+"tokenRedeem"+string("\x00")+redeemTxID+string("\x00")+"0"+string("\x00"))
			Expect(err).NotTo(HaveOccurred())

			output := &token.PlainOutput{}
			err = proto.Unmarshal(po, output)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(Equal(&token.PlainOutput{
				Type:     "TOK1",
				Quantity: 111,
			}))
		})

		It("processes a redeem transaction with some tokens redeemed", func() {
			// prepare redeemTransaction with 2 outputs: one for redeemed tokens and another for remaining tokens
			redeemTransaction = &token.TokenTransaction{
				Action: &token.TokenTransaction_PlainAction{
					PlainAction: &token.PlainTokenAction{
						Data: &token.PlainTokenAction_PlainRedeem{
							PlainRedeem: &token.PlainTransfer{
								Inputs: tokenIds,
								Outputs: []*token.PlainOutput{
									{Type: "TOK1", Quantity: 99},
									{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK1", Quantity: 12},
								},
							},
						},
					},
				},
			}

			err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
			Expect(err).NotTo(HaveOccurred())

			// verify we can get 1 output from "tokenRedeem" and 1 output from "tokenOutput" for this transaction
			po, err := memoryLedger.GetState(tokenNamespace, string("\x00")+"tokenRedeem"+string("\x00")+redeemTxID+string("\x00")+"0"+string("\x00"))
			Expect(err).NotTo(HaveOccurred())

			output := &token.PlainOutput{}
			err = proto.Unmarshal(po, output)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(Equal(&token.PlainOutput{
				Type:     "TOK1",
				Quantity: 99,
			}))

			po, err = memoryLedger.GetState(tokenNamespace, string("\x00")+"tokenOutput"+string("\x00")+redeemTxID+string("\x00")+"1"+string("\x00"))
			Expect(err).NotTo(HaveOccurred())

			err = proto.Unmarshal(po, output)
			Expect(err).NotTo(HaveOccurred())

			Expect(output).To(Equal(&token.PlainOutput{
				Owner:    &token.TokenOwner{Raw: []byte("owner-1")},
				Type:     "TOK1",
				Quantity: 12,
			}))

			spentMarker, err := memoryLedger.GetState(tokenNamespace, string("\x00")+"tokenInput"+string("\x00")+"0"+string("\x00")+"0"+string("\x00"))
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes.Equal(spentMarker, plain.TokenInputSpentMarker)).To(BeTrue())
		})

		Context("when an input has already been spent", func() {
			BeforeEach(func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an InvalidTxError", func() {
				err := verifier.ProcessTx("r2", fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "input with ID \x00tokenOutput\x000\x000\x00 for transfer has already been spent"}))
			})
		})

		Context("when token sum mismatches in inputs and outputs", func() {
			BeforeEach(func() {
				redeemTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainRedeem{
								PlainRedeem: &token.PlainTransfer{
									Inputs: tokenIds,
									Outputs: []*token.PlainOutput{
										{Type: "TOK1", Quantity: 100},
									},
								},
							},
						},
					},
				}
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{
					Msg: fmt.Sprintf("token sum mismatch in inputs and outputs for transaction ID %s (%d vs %d)", redeemTxID, 100, 111)}))
			})
		})

		Context("when inputs have more than one type", func() {
			var (
				anotherImportTransaction *token.TokenTransaction
				anotherImportTxID        string
			)
			BeforeEach(func() {
				anotherImportTxID = "2"
				anotherImportTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainImport{
								PlainImport: &token.PlainImport{
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("owner-1")}, Type: "TOK2", Quantity: 222},
									},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(anotherImportTxID, fakePublicInfo, anotherImportTransaction, memoryLedger)
				Expect(err).NotTo(HaveOccurred())

				redeemTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainRedeem{
								PlainRedeem: &token.PlainTransfer{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
										{TxId: "2", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Type: "TOK1", Quantity: 300},
									},
								},
							},
						},
					},
				}
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{
					Msg: fmt.Sprintf("multiple token types in input for txID: %s (TOK1, TOK2)", redeemTxID)}))
			})
		})

		Context("when redeem output has wrong type", func() {
			BeforeEach(func() {
				redeemTransaction.GetPlainAction().GetPlainRedeem().Outputs[0].Type = "newtype"
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).To(MatchError(fmt.Sprintf(
					fmt.Sprintf("token type mismatch in inputs and outputs for transaction ID %s (%s vs %s)", redeemTxID, "newtype", "TOK1"))))
			})
		})

		Context("when output for remaining tokens has wrong owner", func() {
			BeforeEach(func() {
				// set wrong owner in the output for unredeemed tokens
				redeemTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainRedeem{
								PlainRedeem: &token.PlainTransfer{
									Inputs: tokenIds,
									Outputs: []*token.PlainOutput{
										{Type: "TOK1", Quantity: 99},
										{Owner: &token.TokenOwner{Raw: []byte("owner-2")}, Type: "TOK1", Quantity: 12},
									},
								},
							},
						},
					},
				}
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).To(MatchError(fmt.Sprintf(fmt.Sprintf("wrong owner for remaining tokens, should be original owner owner-1, but got owner-2"))))
			})
		})

		Context("when output for remaining tokens has no owner", func() {
			BeforeEach(func() {
				// do not set owner in the output for unredeemed tokens
				redeemTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainRedeem{
								PlainRedeem: &token.PlainTransfer{
									Inputs: tokenIds,
									Outputs: []*token.PlainOutput{
										{Type: "TOK1", Quantity: 99},
										{Owner: &token.TokenOwner{Raw: []byte("wrong-owner")}, Type: "TOK1", Quantity: 12},
									},
								},
							},
						},
					},
				}
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).To(MatchError(fmt.Sprintf(fmt.Sprintf("wrong owner for remaining tokens, should be original owner owner-1, but got wrong-owner"))))
			})
		})

		Context("when output for redeemed tokens has owner", func() {
			BeforeEach(func() {
				// set owner for the redeem output
				redeemTransaction.GetPlainAction().GetPlainRedeem().Outputs[0].Owner = &token.TokenOwner{Raw: []byte("Owner-1")}
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, memoryLedger)
				Expect(err).To(MatchError(fmt.Sprintf(fmt.Sprintf("owner should be nil in a redeem output"))))
			})
		})

		Context("when redeem output key already exists", func() {
			BeforeEach(func() {
				fakeLedger.GetStateReturns([]byte("state-bytes"), nil)
			})

			It("returns an error", func() {
				err := verifier.ProcessTx(redeemTxID, fakePublicInfo, redeemTransaction, fakeLedger)
				existingOutputID := string("\x00") + "tokenRedeem" + string("\x00") + redeemTxID + string("\x00") + "0" + string("\x00")
				Expect(err).To(MatchError(fmt.Sprintf("output already exists: %s", existingOutputID)))
			})
		})
	})

	Describe("Test ProcessTx PlainApprove", func() {
		var (
			approveTransaction *token.TokenTransaction
			approveTxID        string
			inputBytes         []byte
		)

		BeforeEach(func() {
			approveTxID = "1"
			approveTransaction = &token.TokenTransaction{
				Action: &token.TokenTransaction_PlainAction{
					PlainAction: &token.PlainTokenAction{
						Data: &token.PlainTokenAction_PlainApprove{
							PlainApprove: &token.PlainApprove{
								Inputs: []*token.TokenId{
									{TxId: "0", Index: 0},
								},
								DelegatedOutputs: []*token.PlainDelegatedOutput{
									{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "XYZ", Quantity: 100},
									{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "XYZ", Quantity: 200},
								},
								Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("credential")}, Type: "XYZ", Quantity: 50},
							},
						},
					},
				},
			}
			input := &token.PlainOutput{
				Owner:    &token.TokenOwner{Raw: []byte("credential")},
				Type:     "XYZ",
				Quantity: 350,
			}
			var err error
			inputBytes, err = proto.Marshal(input)
			Expect(err).NotTo(HaveOccurred())

			fakePublicInfo.PublicReturns([]byte("credential"))
			fakeLedger = &mockledger.LedgerWriter{}

			fakeLedger.GetStateReturnsOnCall(6, inputBytes, nil)
		})

		Context("when a valid approve is provided", func() {
			It("is processed successfully", func() {
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the inputs are already spent", func() {
			It("returns an InvalidTxError", func() {
				fakeLedger.GetStateReturnsOnCall(7, []byte("it is spent"), nil)
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "input with ID \x00tokenOutput\x000\x000\x00 for transfer has already been spent"}))
			})
		})

		Context("when a non-existent input is referenced", func() {
			It("returns an InvalidTxError", func() {
				fakeLedger.GetStateReturnsOnCall(6, nil, nil)
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "input with ID \x00tokenOutput\x000\x000\x00 for transfer does not exist"}))
			})
		})

		Context("when the creator of the approve transaction is not the owner of the input", func() {
			It("returns an InvalidTxError", func() {
				approveTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainApprove{
								PlainApprove: &token.PlainApprove{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									DelegatedOutputs: []*token.PlainDelegatedOutput{
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "XYZ", Quantity: 200},
									},
									Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "XYZ", Quantity: 50},
								},
							},
						},
					},
				}
				fakePublicInfo.PublicReturns([]byte("Alice"))
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "transfer input with ID \x00tokenOutput\x000\x000\x00 not owned by creator"}))
			})
		})

		Context("when the same input is spent twice within the same tx", func() {
			It("returns an InvalidTxError", func() {
				fakeLedger.GetStateReturnsOnCall(8, inputBytes, nil)

				approveTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainApprove{
								PlainApprove: &token.PlainApprove{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
										{TxId: "0", Index: 0},
									},
									DelegatedOutputs: []*token.PlainDelegatedOutput{
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "XYZ", Quantity: 200},
									},
									Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("credential")}, Type: "XYZ", Quantity: 50},
								},
							},
						},
					},
				}

				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token input '\x00tokenOutput\x000\x000\x00' spent more than once in transaction ID '1'"}))
			})
		})

		Context("when the input type does not match the output type", func() {
			It("returns an InvalidTxError", func() {
				approveTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainApprove{
								PlainApprove: &token.PlainApprove{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									DelegatedOutputs: []*token.PlainDelegatedOutput{
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "ABC", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "ABC", Quantity: 200},
									},
									Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("credential")}, Type: "ABC", Quantity: 50},
								},
							},
						},
					},
				}

				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token type mismatch in inputs and outputs for approve with ID 1 (ABC vs XYZ)"}))
			})
		})

		Context("when the input sum does not match the output sum", func() {
			It("returns an InvalidTxError", func() {
				approveTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainApprove{
								PlainApprove: &token.PlainApprove{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									DelegatedOutputs: []*token.PlainDelegatedOutput{
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "XYZ", Quantity: 200},
									},
									Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("credential")}, Type: "XYZ", Quantity: 70},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token sum mismatch in inputs and outputs for approve with ID 1 (370 vs 350)"}))
			})
		})

		Context("when the input contains multiple token types", func() {
			It("returns an InvalidTxError", func() {
				input := &token.PlainOutput{
					Owner:    &token.TokenOwner{Raw: []byte("credential")},
					Type:     "ABC",
					Quantity: 100,
				}
				var err error
				inputBytes, err = proto.Marshal(input)
				Expect(err).NotTo(HaveOccurred())
				fakeLedger.GetStateReturnsOnCall(8, inputBytes, nil)

				approveTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainApprove{
								PlainApprove: &token.PlainApprove{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
										{TxId: "0", Index: 0},
									},
									DelegatedOutputs: []*token.PlainDelegatedOutput{
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "XYZ", Quantity: 200},
									},
									Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("credential")}, Type: "XYZ", Quantity: 70},
								},
							},
						},
					},
				}
				err = verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types in input for txID: 1 (XYZ, ABC)"}))
			})
		})

		Context("when the shared outputs contain multiple token types", func() {
			It("returns an InvalidTxError", func() {
				approveTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainApprove{
								PlainApprove: &token.PlainApprove{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									DelegatedOutputs: []*token.PlainDelegatedOutput{
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "ABC", Quantity: 200},
									},
									Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("credential")}, Type: "XYZ", Quantity: 50},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types ('XYZ', 'ABC') in approve outputs for txID '1'"}))
			})
		})
		Context("when the shared outputs and output contain multiple token types", func() {
			It("returns an InvalidTxError", func() {
				approveTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainApprove{
								PlainApprove: &token.PlainApprove{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									DelegatedOutputs: []*token.PlainDelegatedOutput{
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Alice")}}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("credential")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Bob")}}, Type: "XYZ", Quantity: 200},
									},
									Output: &token.PlainOutput{Owner: &token.TokenOwner{Raw: []byte("credential")}, Type: "ABC", Quantity: 50},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types ('ABC', 'XYZ') in approve outputs for txID '1'"}))
			})
		})

		Context("when an output already exists", func() {
			It("returns an error", func() {
				fakeLedger.GetStateReturnsOnCall(0, []byte("an output is already here"), nil)
				err := verifier.ProcessTx(approveTxID, fakePublicInfo, approveTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				existingOutputId := string("\x00") + "tokenOutput" + string("\x00") + "1" + string("\x00") + "0" + string("\x00")
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: fmt.Sprintf("output already exists: %s", existingOutputId)}))
			})
		})
	})
	Describe("Test PlainTransferFrom", func() {
		var (
			transferFromTransaction *token.TokenTransaction
			transferFromTxID        string
			inputBytes              []byte
		)

		BeforeEach(func() {
			transferFromTxID = "1"
			transferFromTransaction = &token.TokenTransaction{
				Action: &token.TokenTransaction_PlainAction{
					PlainAction: &token.PlainTokenAction{
						Data: &token.PlainTokenAction_PlainTransfer_From{
							PlainTransfer_From: &token.PlainTransferFrom{
								Inputs: []*token.TokenId{
									{TxId: "0", Index: 0},
								},
								Outputs: []*token.PlainOutput{
									{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "XYZ", Quantity: 100},
									{Owner: &token.TokenOwner{Raw: []byte("Bob")}, Type: "XYZ", Quantity: 200},
								},
								DelegatedOutput: &token.PlainDelegatedOutput{Owner: &token.TokenOwner{Raw: []byte("owner")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}}, Type: "XYZ", Quantity: 50},
							},
						},
					},
				},
			}
			input := &token.PlainDelegatedOutput{
				Owner:      &token.TokenOwner{Raw: []byte("owner")},
				Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}},
				Type:       "XYZ",
				Quantity:   350,
			}
			var err error
			inputBytes, err = proto.Marshal(input)
			Expect(err).NotTo(HaveOccurred())

			fakePublicInfo.PublicReturns([]byte("Charlie"))
			fakeLedger = &mockledger.LedgerWriter{}

			fakeLedger.GetStateReturnsOnCall(0, inputBytes, nil)
		})

		Context("when a valid transferFrom is provided", func() {
			It("is processed successfully", func() {
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the number of owners of delegated inputs is not correct", func() {
			It("returns an InvalidTxError", func() {
				input := &token.PlainDelegatedOutput{
					Owner:      &token.TokenOwner{Raw: []byte("Alice")},
					Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}, {Raw: []byte("Bob")}},
					Type:       "XYZ",
					Quantity:   350,
				}
				var err error
				inputBytes, err = proto.Marshal(input)
				Expect(err).NotTo(HaveOccurred())

				fakeLedger.GetStateReturnsOnCall(0, inputBytes, nil)
				err = verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "the number of delegatees of delegated input is different from 1: it is 2"}))
			})
		})

		Context("when the delegated inputs are already spent", func() {
			It("returns an InvalidTxError", func() {
				fakeLedger.GetStateReturnsOnCall(1, []byte("it is spent"), nil)
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "delegated input with ID \x00tokenDelegatedOutput\x000\x000\x00 for transferFrom has already been spent"}))
			})
		})

		Context("when a non-existent input is referenced", func() {
			It("returns an InvalidTxError", func() {
				fakeLedger.GetStateReturnsOnCall(0, nil, nil)
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "delegated input with ID \x00tokenDelegatedOutput\x000\x000\x00 for transferFrom does not exist"}))
			})
		})

		Context("when the creator of the transferFrom transaction is not allowed to spend it", func() {
			It("returns an InvalidTxError", func() {
				fakePublicInfo.PublicReturns([]byte("pineapple"))
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "transferFrom input with ID \x00tokenDelegatedOutput\x000\x000\x00 cannot be spent by creator"}))
			})
		})

		Context("when the same input is spent twice within the same tx", func() {
			It("returns an InvalidTxError", func() {
				fakeLedger.GetStateReturnsOnCall(2, inputBytes, nil)

				transferFromTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer_From{
								PlainTransfer_From: &token.PlainTransferFrom{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("Bob")}, Type: "XYZ", Quantity: 200},
									},
									DelegatedOutput: &token.PlainDelegatedOutput{Owner: &token.TokenOwner{Raw: []byte("owner")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}}, Type: "XYZ", Quantity: 50},
								},
							},
						},
					},
				}

				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token delegated input '\x00tokenDelegatedOutput\x000\x000\x00' spent more than once in single transferFrom with txID '1'"}))
			})
		})

		Context("when the input type does not match the output type", func() {
			It("returns an InvalidTxError", func() {
				transferFromTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer_From{
								PlainTransfer_From: &token.PlainTransferFrom{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "ABC", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("Bob")}, Type: "ABC", Quantity: 200},
									},
									DelegatedOutput: &token.PlainDelegatedOutput{Owner: &token.TokenOwner{Raw: []byte("owner")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}}, Type: "ABC", Quantity: 50},
								},
							},
						},
					},
				}

				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token type mismatch in inputs and outputs for transferFrom with ID 1 (ABC vs XYZ)"}))
			})
		})

		Context("when the input sum does not match the output sum", func() {
			It("returns an InvalidTxError", func() {
				transferFromTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer_From{
								PlainTransfer_From: &token.PlainTransferFrom{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("Bob")}, Type: "XYZ", Quantity: 200},
									},
									DelegatedOutput: &token.PlainDelegatedOutput{Owner: &token.TokenOwner{Raw: []byte("owner")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}}, Type: "XYZ", Quantity: 70},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "token sum mismatch in inputs and outputs for transferFrom with ID 1 (370 vs 350)"}))
			})
		})

		Context("when the input contains multiple token types", func() {
			It("returns an InvalidTxError", func() {
				input := &token.PlainDelegatedOutput{
					Owner:      &token.TokenOwner{Raw: []byte("owner")},
					Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}},
					Type:       "ABC",
					Quantity:   100,
				}
				var err error
				inputBytes, err = proto.Marshal(input)
				Expect(err).NotTo(HaveOccurred())
				fakeLedger.GetStateReturnsOnCall(2, inputBytes, nil)

				transferFromTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer_From{
								PlainTransfer_From: &token.PlainTransferFrom{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("Bob")}, Type: "XYZ", Quantity: 200},
									},
									DelegatedOutput: &token.PlainDelegatedOutput{Owner: &token.TokenOwner{Raw: []byte("owner")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}}, Type: "XYZ", Quantity: 70},
								},
							},
						},
					},
				}
				err = verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types in transferFrom input for txID: 1 (XYZ, ABC)"}))
			})
		})

		Context("when the outputs contain multiple token types", func() {
			It("returns an InvalidTxError", func() {
				transferFromTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer_From{
								PlainTransfer_From: &token.PlainTransferFrom{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("Bob")}, Type: "ABC", Quantity: 200},
									},
									DelegatedOutput: &token.PlainDelegatedOutput{Owner: &token.TokenOwner{Raw: []byte("owner")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}}, Type: "XYZ", Quantity: 50},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types ('XYZ', 'ABC') in transferFrom outputs for txID '1'"}))
			})
		})

		Context("when the shared output and outputs contain multiple token types", func() {
			It("returns an InvalidTxError", func() {
				transferFromTransaction = &token.TokenTransaction{
					Action: &token.TokenTransaction_PlainAction{
						PlainAction: &token.PlainTokenAction{
							Data: &token.PlainTokenAction_PlainTransfer_From{
								PlainTransfer_From: &token.PlainTransferFrom{
									Inputs: []*token.TokenId{
										{TxId: "0", Index: 0},
									},
									Outputs: []*token.PlainOutput{
										{Owner: &token.TokenOwner{Raw: []byte("Alice")}, Type: "XYZ", Quantity: 100},
										{Owner: &token.TokenOwner{Raw: []byte("Bob")}, Type: "XYZ", Quantity: 200},
									},
									DelegatedOutput: &token.PlainDelegatedOutput{Owner: &token.TokenOwner{Raw: []byte("owner")}, Delegatees: []*token.TokenOwner{{Raw: []byte("Charlie")}}, Type: "ABC", Quantity: 50},
								},
							},
						},
					},
				}
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: "multiple token types ('ABC', 'XYZ') in transferFrom outputs for txID '1'"}))
			})
		})

		Context("when a shared output already exists", func() {
			It("returns an error", func() {
				fakeLedger.GetStateReturnsOnCall(2, []byte("a delegated output is already here"), nil)
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				existingOutputId := string("\x00") + "tokenDelegatedOutput" + string("\x00") + "1" + string("\x00") + "0" + string("\x00")
				Expect(err.Error()).To(Equal(fmt.Sprintf("delegated output already exists: %s", existingOutputId)))
			})
		})

		Context("when an output already exists", func() {
			It("returns an error", func() {
				fakeLedger.GetStateReturnsOnCall(4, []byte("an output is already here"), nil)
				err := verifier.ProcessTx(transferFromTxID, fakePublicInfo, transferFromTransaction, fakeLedger)
				Expect(err).To(HaveOccurred())
				existingOutputId := string("\x00") + "tokenOutput" + string("\x00") + "1" + string("\x00") + "0" + string("\x00")
				Expect(err).To(Equal(&customtx.InvalidTxError{Msg: fmt.Sprintf("output already exists: %s", existingOutputId)}))
			})
		})

	})
})
