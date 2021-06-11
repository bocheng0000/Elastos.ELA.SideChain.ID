package blockchain

import (
	"bytes"
	"strconv"
	"strings"
	"sync"

	idtypes "github.com/elastos/Elastos.ELA.SideChain.ID/types"
	"github.com/elastos/Elastos.ELA.SideChain/types"
	elacom "github.com/elastos/Elastos.ELA/common"
	"github.com/elastos/Elastos.ELA/common/log"
	"github.com/elastos/Elastos.ELA/core/types/payload"
)

var MemPoolEx MemPool

type MemPool struct {
	i    int
	c    IChainStoreExtend
	is_p map[elacom.Uint256]bool
	p    map[string][]byte
	l    sync.RWMutex
}

func (m *MemPool) AppendToMemPool(tx *types.Transaction) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered from AppendToMemPool ", r)
		}
	}()
	m.l.RLock()
	if _, ok := m.is_p[tx.Hash()]; ok {
		m.l.RUnlock()
		return nil
	}
	m.l.RUnlock()
	txhs := make([]idtypes.TransactionHistory, 0)

	var memo []byte
	var txType = tx.TxType
	for _, attr := range tx.Attributes {
		if attr.Usage == types.Memo {
			memo = attr.Data
		}
	}

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
		referTx, _, err := m.c.GetTransaction(txid)
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
	for k, r := range receive {
		transferType := RECEIVED
		s, ok := spend[k]
		var value elacom.Fixed64
		if ok {
			if s > r {
				value = s - r
				transferType = SENT
			} else {
				value = r - s
			}
			delete(spend, k)
		} else {
			value = r
		}
		var realFee = fee
		var rto = toAddress
		if transferType == RECEIVED {
			realFee = 0
			rto = []elacom.Uint168{k}
		}

		if transferType == SENT {
			fromAddress = []elacom.Uint168{k}
		}

		txh := idtypes.TransactionHistory{}
		txh.Value = value
		txh.Address = k
		txh.Inputs = fromAddress
		txh.TxType = txType
		txh.Txid = tx.Hash()
		txh.Height = 0
		txh.Time = 0
		txh.Type = []byte(transferType)
		txh.Fee = realFee
		if len(rto) > 10 {
			txh.Outputs = rto[0:10]
		} else {
			txh.Outputs = rto
		}
		txh.Memo = memo
		txh.Status = 1
		txhs = append(txhs, txh)
	}

	for addr, value := range spend {
		txh := idtypes.TransactionHistory{}
		txh.Value = value
		txh.Address = addr
		txh.Inputs = []elacom.Uint168{addr}
		txh.TxType = txType
		txh.Txid = tx.Hash()
		txh.Height = 0
		txh.Time = 0
		txh.Type = []byte(SENT)
		txh.Fee = fee
		if len(toAddress) > 10 {
			txh.Outputs = toAddress[0:10]
		} else {
			txh.Outputs = toAddress
		}
		txh.Memo = memo
		txh.Status = 1
		txhs = append(txhs, txh)
	}
	for _, p := range txhs {
		m.l.Lock()
		m.is_p[p.Txid] = true
		m.l.Unlock()
		m.i += m.i
		err := m.store(p.Txid, p)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MemPool) store(txid elacom.Uint256, history idtypes.TransactionHistory) error {
	m.l.Lock()
	defer m.l.Unlock()
	addr, _ := history.Address.ToAddress()
	value := new(bytes.Buffer)
	history.Serialize(value)
	m.p[addr+txid.String()+strconv.Itoa(m.i)] = value.Bytes()
	return nil
}

func (m *MemPool) GetMemPoolTx(address *elacom.Uint168) (ret []idtypes.TransactionHistoryDisplay) {
	m.l.RLock()
	defer m.l.RUnlock()
	for k, v := range m.p {
		addr, err := address.ToAddress()
		if err != nil {
			log.Warnf("Warn invalid address %s", addr)
			return
		}
		if strings.Contains(k, addr) {
			buf := new(bytes.Buffer)
			buf.Write(v)
			txh := idtypes.TransactionHistory{}
			txhd, _ := txh.Deserialize(buf)
			ret = append(ret, *txhd)
		}
	}

	return
}

func (m *MemPool) DeleteMemPoolTx(txid elacom.Uint256) {
	m.l.Lock()
	defer m.l.Unlock()
	for k := range m.p {
		if strings.Contains(k, txid.String()) {
			delete(m.p, k)
			delete(m.is_p, txid)
		}
	}
}
