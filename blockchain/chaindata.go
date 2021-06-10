package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/elastos/Elastos.ELA.SideChain.ID/types"
	"github.com/elastos/Elastos.ELA/common"
	"github.com/elastos/Elastos.ELA/common/log"
)

func (c ChainStoreExtend) begin() {
	c.NewBatch()
}

func (c ChainStoreExtend) commit() {
	//c.BatchCommit()
	//batch := c.NewBatch()
	//batch.Commit()
	//c.commit()
}

func (c ChainStoreExtend) rollback() {

}

// key: DataEntryPrefix + height + address
// value: serialized history
func (c ChainStoreExtend) persistTransactionHistory(txhs []types.TransactionHistory) error {
	fmt.Println()
	fmt.Println("###### 4-start func (c ChainStoreExtend) persistTransactionHistory(txhs []types.TransactionHistory) ######")
	fmt.Println("txhs:", txhs)

	c.begin()
	for i, txh := range txhs {
		fmt.Println("###### 4.1 go into c.doPersistTransactionHistory(uint64(i), txh) ######")
		err := c.doPersistTransactionHistory(uint64(i), txh)
		if err != nil {
			c.rollback()
			log.Fatal("Error persist transaction history")
			return err
		}
	}
	for _, txh := range txhs {
		c.deleteMemPoolTx(txh.Txid)
	}
	c.commit()
	//key := new(bytes.Buffer)
	//key.WriteByte(byte(DataTxHistoryPrefix))
	//if len(txhs) > 0 {
	//	fmt.Println("#### 4.3 ####")
	//	fmt.Println("Txid:",txhs[0].Txid)
	//	fmt.Println("Height:",txhs[0].Height)
	//	fmt.Println("Address:",txhs[0].Address)
	//	fmt.Println("##### 4.3 end ###")
	//
	//	err := common.WriteVarBytes(key, txhs[0].Address[:])
	//	if err != nil {
	//		return err
	//	}
	//	err = common.WriteUint64(key, txhs[0].Height)
	//	_result, err := c.Get(key.Bytes())
	//
	//	val := new(bytes.Buffer)
	//	val.Write(_result)
	//	txh := types.TransactionHistory{}
	//	txhd, _ := txh.Deserialize(val)
	//	fmt.Println("##### 4.4 txhd:", txhd.Txid,"#", txhd.Address,"#", txhd.Height,"#")
	//}

	fmt.Println("###### 4-end func (c ChainStoreExtend) persistTransactionHistory(txhs []types.TransactionHistory) ######")
	return nil
}

func (c ChainStoreExtend) deleteMemPoolTx(txid common.Uint256) {
	MemPoolEx.DeleteMemPoolTx(txid)
}

func (c ChainStoreExtend) persistBestHeight(height uint32) error {
	bestHeight, err := c.GetBestHeightExt()
	if (err == nil && bestHeight < height) || err != nil {
		c.begin()
		err = c.doPersistBestHeight(height)
		if err != nil {
			c.rollback()
			log.Fatal("Error persist best height")
			return err
		}
		c.commit()
	}
	return nil
}

func (c ChainStoreExtend) doPersistBestHeight(height uint32) error {
	key := new(bytes.Buffer)
	key.WriteByte(byte(DataBestHeightPrefix))
	value := new(bytes.Buffer)
	common.WriteUint32(value, height)
	//c.BatchPut(key.Bytes(), value.Bytes())
	batch := c.NewBatch()
	return batch.Put(key.Bytes(), value.Bytes())
}

func (c ChainStoreExtend) persistStoredHeight(height uint32) error {
	c.begin()
	err := c.doPersistStoredHeight(height)
	if err != nil {
		c.rollback()
		log.Fatal("Error persist best height")
		return err
	}
	c.commit()
	return nil
}

func (c ChainStoreExtend) doPersistStoredHeight(height uint32) error {
	key := new(bytes.Buffer)
	key.WriteByte(byte(DataStoredHeightPrefix))
	common.WriteUint32(key, height)
	value := new(bytes.Buffer)
	common.WriteVarBytes(value, []byte{1})
	//c.BatchPut(key.Bytes(), value.Bytes())
	batch := c.NewBatch()
	return batch.Put(key.Bytes(), value.Bytes())
}

func (c ChainStoreExtend) doPersistTransactionHistory(i uint64, history types.TransactionHistory) error {
	fmt.Println("$$$$$$ 5-start func (c ChainStoreExtend) doPersistTransactionHistory(i uint64, history types.TransactionHistory) $$$$$$")

	fmt.Println("history:", history)
	fmt.Println("Txid:", history.Txid)
	fmt.Println("Address:", history.Address)
	fmt.Println("Height:", history.Height)
	key := new(bytes.Buffer)
	key.WriteByte(byte(DataTxHistoryPrefix))
	err := common.WriteVarBytes(key, history.Address[:])
	if err != nil {
		return err
	}
	err = common.WriteUint64(key, history.Height)
	if err != nil {
		return err
	}
	err = common.WriteUint64(key, i)
	if err != nil {
		return err
	}

	value := new(bytes.Buffer)
	err = history.Serialize(value)
	value1 := bytes.NewBuffer(value.Bytes())

	fmt.Println("$$ 5.0 history.Serialize(value)")
	fmt.Println("Err", err)
	fmt.Println("value", hex.EncodeToString(value.Bytes()))
	fmt.Println("value1", hex.EncodeToString(value1.Bytes()))


	_result, err := history.Deserialize(value1)
	fmt.Println("$$ 5. history.Deserialize(value)")
	fmt.Println("Err", err)
	fmt.Println("Txid", _result.Txid)
	fmt.Println("Height", _result.Height)
	fmt.Println("Address", _result.Address)

	fmt.Println("$$ 5. history.Deserialize(value)-end")

	//c.BatchPut(key.Bytes(), value.Bytes())
	fmt.Println("err returned by history.Serialize:", err)
	fmt.Println("history.Serialize(value)", hex.EncodeToString(value.Bytes()))
	//batch := c.NewBatch()
	err = c.Put(key.Bytes(), value.Bytes())
	//c.commit()

	key1 := new(bytes.Buffer)
	key1.WriteByte(byte(DataTxHistoryPrefix))

	//fmt.Println("#### 4.3 ####")
	//fmt.Println("Txid:",txhs[0].Txid)
	//fmt.Println("Height:",txhs[0].Height)
	//fmt.Println("Address:",txhs[0].Address)
	//fmt.Println("##### 4.3 end ###")

	//err = common.WriteVarBytes(key1, history.Address[:])
	//if err != nil {
	//	return err
	//}
	//err = common.WriteUint64(key1, history.Height)
	_result1, err := c.Get(key.Bytes())

	val := new(bytes.Buffer)
	val.Write(_result1)
	txh := types.TransactionHistory{}
	txhd, err := txh.Deserialize(val)
	fmt.Println("##### 4.4 txhd:", txhd.Txid, "#", txhd.Address, "#", txhd.Height, "#")
	fmt.Println("_result1:", hex.EncodeToString(_result1))
	fmt.Println("txh.Deserialize ERR:", err)

	fmt.Println("$$$$$$ 5 batch.put-err", err, " $$$$$$")
	fmt.Println("$$$$$$ 5-end func (c ChainStoreExtend) doPersistTransactionHistory(i uint64, history types.TransactionHistory) $$$$$$")
	return err
}
