package synchroniser

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/db"
	L "QRLtoMongoDB-PoS/logger"
	"QRLtoMongoDB-PoS/models"
	"QRLtoMongoDB-PoS/rpc"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	"go.uber.org/zap"
)

var logger *zap.Logger = L.FileLogger(configs.Filename)

type Data struct {
	blockData    []interface{}
	blockNumbers []int
}

func Sync() {
	var bNum uint64 = db.GetLatestBlockNumberFromDB() + 1

	wg := sync.WaitGroup{}

	max, err := rpc.GetLatestBlock()
	if err != nil {
		log.Fatal(err)
	}

	// Create a buffered channel of read only channels, with length 8.
	producers := make(chan (<-chan Data), 8)

	// Start the consumer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer(os.Stdout, producers)
	}()

	// Start producers in correct order.
	for i, size := int(bNum), 8; i < int(max); i += size {
		// Send the producer's channel over to the consumer.
		producers <- producer(i, i+size)
	}
	// No more producers will be started, so close the channel.
	close(producers)

	// Wait for consumer!
	wg.Wait()
	fmt.Println("Done!")

	db.GetDailyTransactionVolume()

	singleBlockInsertion()

	// rpc.ZondGetLogs("0xcEF0271647F8887358e00527aB8D9205a87F5fd3")
}

func consumer(w io.Writer, ch <-chan (<-chan Data)) {
	// Consume the producer channels.
	for Datas := range ch {
		// Consume the Datas.
		for i := range Datas {
			// Do stuff with the Datas, in order.
			db.InsertManyBlockDocuments(i.blockData)
			// db.ProcessAverageBlockSize(i.blockData)
			for x := 0; x < len(i.blockNumbers); x++ {
				// db.ProcessCreditingFromBlockData(i.blockData[x])
				db.ProcessTransactions(i.blockData[x])
			}
		}
	}
}

func producer(start int, end int) <-chan Data {
	// Create a channel which we will send our data.
	Datas := make(chan Data, 8)

	var blockData []interface{}
	var blockNumbers []int

	// Start the goroutine that produces data.
	go func(ch chan<- Data) {
		defer close(ch)

		// Produce data.
		for i := start; i < end; i++ {
			data := rpc.GetBlockByNumberMainnet(uint64(i))

			var Zond models.Zond
			var ZondNew models.ZondDatabaseBlock

			json.Unmarshal([]byte(data), &Zond)

			if Zond.PreResult.ParentHash != "" {
				ZondNew = db.ConvertModelsUint64(Zond)
				blockData = append(blockData, ZondNew)
				blockNumbers = append(blockNumbers, i)
			}
		}
		if blockData != nil {
			ch <- Data{blockData: blockData, blockNumbers: blockNumbers}
		}
	}(Datas)

	// Return back to caller.
	return Datas
}

func processInitialBlock() {
	var Zond models.Zond
	var ZondNew models.ZondDatabaseBlock

	blockData := rpc.GetBlockByNumberMainnet(0)

	err := json.Unmarshal([]byte(blockData), &Zond)
	if err != nil {
		glog.Info("%v", err)
		return
	}
	ZondNew = db.ConvertModelsUint64(Zond)
	db.InsertBlockDocument(ZondNew)
	// db.ProcessSingleCreditingFromBlockData(ZondNew)
	db.ProcessTransactions(ZondNew)
}

func processSubsequentBlocks(sum uint64, latestBlockNumber uint64) {
	var Zond models.Zond
	var ZondNew models.ZondDatabaseBlock

	blockData := rpc.GetBlockByNumberMainnet(sum)

	err := json.Unmarshal([]byte(blockData), &Zond)
	if err != nil {
		glog.Info("%v", err)
		return
	}

	if Zond.PreResult.ParentHash != "" {
		ZondNew = db.ConvertModelsUint64(Zond)
		previousHash := ZondNew.Result.ParentHash
		if previousHash == db.GetLatestBlockHashHeaderFromDB(db.GetLatestBlockNumberFromDB()) {
			processBlockAndUpdateValidators(sum, ZondNew, previousHash)
		} else {
			db.Rollback(sum)
			sum = sum - 2
		}
	}
}

func processBlockAndUpdateValidators(sum uint64, ZondNew models.ZondDatabaseBlock, previousHash string) {
	db.InsertBlockDocument(ZondNew)
	// db.ProcessSingleCreditingFromBlockData(ZondNew)
	db.ProcessTransactions(ZondNew)
	db.UpdateValidators(sum, previousHash)
}

func singleBlockInsertion() {
	sum := db.GetLatestBlockNumberFromDB() + 1

	latestBlockNumber, err := rpc.GetLatestBlock()
	if err != nil {
		fmt.Println(err)
	}

	isCollectionsExist := db.IsCollectionsExist()

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if isCollectionsExist != true {
				processInitialBlock()
				isCollectionsExist = true
			} else {
				if sum <= latestBlockNumber {
					processSubsequentBlocks(sum, latestBlockNumber)
					sum++
				} else {
					latestBlockNumber, err = rpc.GetLatestBlock()
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			db.PeriodicallyUpdateCoinGeckoData()
			db.CountWallets()
			// db.UpdateTotalBalance()
			db.GetDailyTransactionVolume()
			// db.CalculateAndStoreAverageVolume()
		}
	}()

	select {}
}
