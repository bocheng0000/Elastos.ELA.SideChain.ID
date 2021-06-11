package blockchain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	idtypes "github.com/elastos/Elastos.ELA.SideChain.ID/types"
	"github.com/elastos/Elastos.ELA.SideChain/blockchain"
	"github.com/elastos/Elastos.ELA.SideChain/database"
	"github.com/elastos/Elastos.ELA.SideChain/events"
	"github.com/elastos/Elastos.ELA.SideChain/types"
	elacom "github.com/elastos/Elastos.ELA/common"
	"github.com/elastos/Elastos.ELA/core/types/payload"
	"github.com/elastos/Elastos.ELA/utils"
)

// DataEntryPrefix
type DataEntryPrefix byte

const (
	DataTxHistoryPrefix    DataEntryPrefix = 0x60
	DataBestHeightPrefix   DataEntryPrefix = 0x61
	DataStoredHeightPrefix DataEntryPrefix = 0x62
)

// IChainStore provides func with store package.
type IChainStore interface {
	SaveBlock(b *types.Block) error
	IsDoubleSpend(tx *types.Transaction) bool
	RollbackBlock(blockHash elacom.Uint256) error
	GetTransaction(txID elacom.Uint256) (*types.Transaction, uint32, error)
	GetTxReference(tx *types.Transaction) (map[*types.Input]*types.Output, error)
	GetHeight() uint32
	Close()
}

var (
	StoreEx IChainStoreExtend
)

var (
	MINING_ADDR = elacom.Uint168{}
)

type IStore interface {
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	NewBatch() database.Batch
	Close() error
	NewIterator(prefix []byte) database.Iterator
}

type IChainStoreExtend interface {
	IChainStore
	persistTxHistory(block *types.Block) error
	CloseEx()
	AddTask(task interface{})
	GetTxHistory(addr, order string, timestamp uint64) interface{}
	GetTxHistoryByLimit(addr, order string, skip, limit, timestamp uint32) (interface{}, int)
}

type ChainStoreExtend struct {
	IChainStore
	IStore
	chain      *blockchain.BlockChain
	taskChEx   chan interface{}
	quitEx     chan chan bool
	mu         sync.RWMutex
	rp         chan bool
	checkPoint bool
}

func (c *ChainStoreExtend) AddTask(task interface{}) {
	c.taskChEx <- task
}

func NewChainStoreEx(chain *blockchain.BlockChain, chainstore IChainStore, filePath string) (*ChainStoreExtend, error) {
	if !utils.FileExisted(filePath) {
		os.MkdirAll(filePath, 0700)
	}
	st, err := database.NewLevelDB(filePath)
	if err != nil {
		return nil, err
	}
	c := &ChainStoreExtend{
		IChainStore: chainstore,
		IStore:      st,
		chain:       chain,
		taskChEx:    make(chan interface{}, 100),
		quitEx:      make(chan chan bool, 1),
		mu:          sync.RWMutex{},
		rp:          make(chan bool, 1),
		checkPoint:  true,
	}
	StoreEx = c
	MemPoolEx = MemPool{
		c:    StoreEx,
		is_p: make(map[elacom.Uint256]bool),
		p:    make(map[string][]byte),
	}
	go c.loop()
	events.Subscribe(func(e *events.Event) {
		switch e.Type {
		case events.ETBlockConnected:
			b, ok := e.Data.(*types.Block)
			if ok {
				go StoreEx.AddTask(b)
			}
		case events.ETTransactionAccepted:
			tx, ok := e.Data.(*types.Transaction)
			if ok {
				go MemPoolEx.AppendToMemPool(tx)
			}
		}
	})
	return c, nil
}

func (c *ChainStoreExtend) Close() {

}

func (c *ChainStoreExtend) assembleRollbackBlock(rollbackStart uint32, blk *types.Block, blocks *[]*types.Block) error {
	for i := rollbackStart; i < blk.GetHeight(); i++ {
		blockHash, err := c.chain.GetBlockHash(i)
		if err != nil {
			return err
		}
		b, err := c.chain.GetBlockByHash(blockHash)
		if err != nil {
			return err
		}
		*blocks = append(*blocks, b)
	}
	return nil
}

func (c *ChainStoreExtend) persistTxHistory(blk *types.Block) error {
	var blocks []*types.Block
	var rollbackStart uint32 = 0
	if c.checkPoint {
		bestHeight, err := c.GetBestHeightExt()
		if err == nil && bestHeight > CHECK_POINT_ROLLBACK_HEIGHT {
			rollbackStart = bestHeight - CHECK_POINT_ROLLBACK_HEIGHT
		}
		c.assembleRollbackBlock(rollbackStart, blk, &blocks)
		c.checkPoint = false
	}

	blocks = append(blocks, blk)

	for _, block := range blocks {
		_, err := c.GetStoredHeightExt(block.GetHeight())
		if err == nil {
			continue
		}

		txs := block.Transactions
		txhs := make([]idtypes.TransactionHistory, 0)
		for i := 0; i < len(txs); i++ {
			tx := txs[i]
			var memo []byte
			var txType = tx.TxType
			for _, attr := range tx.Attributes {
				if attr.Usage == types.Memo {
					memo = attr.Data
				}
			}

			if txType == types.CoinBase {
				var to []elacom.Uint168
				hold := make(map[elacom.Uint168]elacom.Fixed64)
				txhsCoinBase := make([]idtypes.TransactionHistory, 0)
				for _, vout := range tx.Outputs {
					if !elacom.ContainsU168(vout.ProgramHash, to) {
						to = append(to, vout.ProgramHash)
						txh := idtypes.TransactionHistory{}
						txh.Address = vout.ProgramHash
						txh.Txid = tx.Hash()
						txh.Type = []byte(RECEIVED)
						txh.Time = uint64(block.Header.GetTimeStamp())
						txh.Height = uint64(block.GetHeight())
						txh.Fee = 0
						txh.Inputs = []elacom.Uint168{MINING_ADDR}
						txh.TxType = txType
						txh.Memo = memo

						hold[vout.ProgramHash] = vout.Value
						txhsCoinBase = append(txhsCoinBase, txh)
					} else {
						hold[vout.ProgramHash] += vout.Value
					}
				}
				for i := 0; i < len(txhsCoinBase); i++ {
					txhsCoinBase[i].Outputs = []elacom.Uint168{txhsCoinBase[i].Address}
					txhsCoinBase[i].Value = hold[txhsCoinBase[i].Address]
				}
				txhs = append(txhs, txhsCoinBase...)
			} else {
				isCrossTx := false
				if txType == types.TransferCrossChainAsset {
					isCrossTx = true
				}
				spend := make(map[elacom.Uint168]elacom.Fixed64)
				var totalInput elacom.Fixed64 = 0
				var fromAddress []elacom.Uint168
				var toAddress []elacom.Uint168
				for _, input := range tx.Inputs {
					txid := input.Previous.TxID
					index := input.Previous.Index
					referTx, _, err := c.GetTransaction(txid)
					if err != nil {
						return err
					}
					address := referTx.Outputs[index].ProgramHash
					totalInput += referTx.Outputs[index].Value
					v, ok := spend[address]
					if ok {
						spend[address] = v + referTx.Outputs[index].Value
					} else {
						spend[address] = referTx.Outputs[index].Value
					}
					if !elacom.ContainsU168(address, fromAddress) {
						fromAddress = append(fromAddress, address)
					}
				}
				receive := make(map[elacom.Uint168]elacom.Fixed64)
				var totalOutput elacom.Fixed64 = 0
				for _, output := range tx.Outputs {
					address, _ := output.ProgramHash.ToAddress()
					var valueCross elacom.Fixed64
					if isCrossTx == true && (output.ProgramHash == MINING_ADDR || strings.Index(address, "X") == 0 || address == "4oLvT2") {
						switch pl := tx.Payload.(type) {
						case *payload.TransferCrossChainAsset:
							valueCross = pl.CrossChainAmounts[0]
						}
					}
					if valueCross != 0 {
						totalOutput += valueCross
					} else {
						totalOutput += output.Value
					}
					v, ok := receive[output.ProgramHash]
					if ok {
						receive[output.ProgramHash] = v + output.Value
					} else {
						receive[output.ProgramHash] = output.Value
					}
					if !elacom.ContainsU168(output.ProgramHash, toAddress) {
						toAddress = append(toAddress, output.ProgramHash)
					}
				}
				fee := totalInput - totalOutput
				for addressReceiver, valueReceived := range receive {
					transferType := RECEIVED
					valueSpent, ok := spend[addressReceiver]
					var txValue elacom.Fixed64
					if ok {
						if valueSpent > valueReceived {
							txValue = valueSpent - valueReceived
							transferType = SENT
						} else {
							txValue = valueReceived - valueSpent
						}
						delete(spend, addressReceiver)
					} else {
						txValue = valueReceived
					}
					var realFee = fee
					var txOutput = toAddress
					if transferType == RECEIVED {
						realFee = 0
						txOutput = []elacom.Uint168{addressReceiver}
					}

					if transferType == SENT {
						fromAddress = []elacom.Uint168{addressReceiver}
					}

					txh := idtypes.TransactionHistory{}
					txh.Value = txValue
					txh.Address = addressReceiver
					txh.Inputs = fromAddress
					txh.TxType = txType
					txh.Txid = tx.Hash()
					txh.Height = uint64(block.GetHeight())
					txh.Time = uint64(block.Header.GetTimeStamp())
					txh.Type = []byte(transferType)
					txh.Fee = realFee
					if len(txOutput) > 10 {
						txh.Outputs = txOutput[0:10]
					} else {
						txh.Outputs = txOutput
					}
					txh.Memo = memo
					txhs = append(txhs, txh)
				}

				for addr, value := range spend {
					txh := idtypes.TransactionHistory{}
					txh.Value = value
					txh.Address = addr
					txh.Inputs = []elacom.Uint168{addr}
					txh.TxType = txType
					txh.Txid = tx.Hash()
					txh.Height = uint64(block.GetHeight())
					txh.Time = uint64(block.Header.GetTimeStamp())
					txh.Type = []byte(SENT)
					txh.Fee = fee
					if len(toAddress) > 10 {
						txh.Outputs = toAddress[0:10]
					} else {
						txh.Outputs = toAddress
					}
					txh.Memo = memo
					txhs = append(txhs, txh)
				}
			}
		}
		c.persistTransactionHistory(txhs)
		c.persistStoredHeight(block.GetHeight())
	}
	return nil
}

func (c *ChainStoreExtend) CloseEx() {
	closed := make(chan bool)
	c.quitEx <- closed
	<-closed
}

func (c *ChainStoreExtend) loop() {
	for {
		select {
		case t := <-c.taskChEx:
			now := time.Now()
			switch kind := t.(type) {
			case *types.Block:
				err := c.persistTxHistory(kind)
				if err != nil {
					fmt.Printf("Error persist transaction history %s", err.Error())
					os.Exit(-1)
					return
				}
				tcall := float64(time.Now().Sub(now)) / float64(time.Second)
				fmt.Printf("handle SaveHistory time cost: %g num transactions:%d", tcall, len(kind.Transactions))
			}
		case closed := <-c.quitEx:
			closed <- true
			return
		}
	}
}

func (c *ChainStoreExtend) GetTxHistory(addr string, order string, timestamp uint64) interface{} {
	key := new(bytes.Buffer)
	key.WriteByte(byte(DataTxHistoryPrefix))
	var txhs interface{}
	if order == "desc" {
		txhs = make(idtypes.TransactionHistorySorterDesc, 0)
	} else {
		txhs = make(idtypes.TransactionHistorySorter, 0)
	}
	programHash, err := elacom.Uint168FromAddress(addr)
	if err != nil {
		return txhs
	}
	elacom.WriteVarBytes(key, programHash[:])
	iter := c.NewIterator(key.Bytes())
	defer iter.Release()

	for iter.Next() {
		val := new(bytes.Buffer)
		val.Write(iter.Value())
		txh := idtypes.TransactionHistory{}
		txhd, _ := txh.Deserialize(val)
		if txhd.Type == "received" {
			if len(txhd.Inputs) > 10 {
				txhd.Inputs = txhd.Inputs[0:10]
			}
			txhd.Outputs = []string{txhd.Address}
		} else {
			txhd.Inputs = []string{txhd.Address}
			if len(txhd.Outputs) > 10 {
				txhd.Outputs = txhd.Outputs[0:10]
			}
		}

		if (timestamp > 0 && txhd.Time > timestamp) || timestamp == 0 {
			if order == "desc" {
				txhs = append(txhs.(idtypes.TransactionHistorySorterDesc), *txhd)
			} else {
				txhs = append(txhs.(idtypes.TransactionHistorySorter), *txhd)
			}
		}
	}

	txInMempool := MemPoolEx.GetMemPoolTx(programHash)
	for _, txh := range txInMempool {
		if order == "desc" {
			txhs = append(txhs.(idtypes.TransactionHistorySorterDesc), txh)
		} else {
			txhs = append(txhs.(idtypes.TransactionHistorySorter), txh)
		}
	}

	if order == "desc" {
		sort.Sort(txhs.(idtypes.TransactionHistorySorterDesc))
	} else {
		sort.Sort(txhs.(idtypes.TransactionHistorySorter))
	}
	return txhs
}

func (c *ChainStoreExtend) GetTxHistoryByLimit(addr, order string, skip, limit, timestamp uint32) (interface{}, int) {
	txhs := c.GetTxHistory(addr, order, uint64(timestamp))
	if order == "desc" {
		return txhs.(idtypes.TransactionHistorySorterDesc).Filter(skip, limit), len(txhs.(idtypes.TransactionHistorySorterDesc))
	} else {
		return txhs.(idtypes.TransactionHistorySorter).Filter(skip, limit), len(txhs.(idtypes.TransactionHistorySorter))
	}
}

func (c *ChainStoreExtend) GetBestHeightExt() (uint32, error) {
	key := new(bytes.Buffer)
	key.WriteByte(byte(DataBestHeightPrefix))
	data, err := c.Get(key.Bytes())
	if err != nil {
		return 0, err
	}
	buf := bytes.NewBuffer(data)
	return binary.LittleEndian.Uint32(buf.Bytes()), nil
}

func (c *ChainStoreExtend) GetStoredHeightExt(height uint32) (bool, error) {
	key := new(bytes.Buffer)
	key.WriteByte(byte(DataStoredHeightPrefix))
	elacom.WriteUint32(key, height)
	_, err := c.Get(key.Bytes())
	if err != nil {
		return false, err
	}
	return true, nil
}
