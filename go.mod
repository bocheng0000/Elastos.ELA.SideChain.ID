module github.com/elastos/Elastos.ELA.SideChain.ID

go 1.13

require (
	github.com/btcsuite/btcutil v0.0.0-20191219182022-e17c9730c422
	github.com/elastos/Elastos.ELA v0.7.0
	github.com/elastos/Elastos.ELA.SPV v0.0.7
	github.com/elastos/Elastos.ELA.SideChain v0.2.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/robfig/cron v1.2.0
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.0
	golang.org/x/crypto v0.0.0-20200604202706-70a84ac30bf9
)

replace github.com/elastos/Elastos.ELA => ../Elastos.ELA

replace github.com/elastos/Elastos.ELA.SideChain => ../Elastos.ELA.SideChain
