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
		log.Printf("Failed to get latest block: %v", err)
		return
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
}

func consumer(w io.Writer, ch <-chan (<-chan Data)) {
	// Consume the producer channels.
	for Datas := range ch {
		// Consume the Datas.
		for i := range Datas {
			if i.blockData == nil || len(i.blockData) == 0 {
				continue
			}
			// Do stuff with the Datas, in order.
			db.InsertManyBlockDocuments(i.blockData)
			for x := 0; x < len(i.blockNumbers); x++ {
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
			if data == "" {
				continue
			}

			var Zond models.Zond
			err := json.Unmarshal([]byte(data), &Zond)
			if err != nil {
				continue
			}

			if Zond.PreResult.ParentHash != "" {
				ZondNew := db.ConvertModelsUint64(Zond)
				blockData = append(blockData, ZondNew)
				blockNumbers = append(blockNumbers, i)
			}
		}
		if len(blockData) > 0 {
			ch <- Data{blockData: blockData, blockNumbers: blockNumbers}
		}
	}(Datas)

	// Return back to caller.
	return Datas
}

func processInitialBlock() {
	blockData := rpc.GetBlockByNumberMainnet(0)
	if blockData == "" {
		return
	}

	var Zond models.Zond
	err := json.Unmarshal([]byte(blockData), &Zond)
	if err != nil {
		glog.Info("%v", err)
		return
	}
	ZondNew := db.ConvertModelsUint64(Zond)
	db.InsertBlockDocument(ZondNew)
	db.ProcessTransactions(ZondNew)
}

func processSubsequentBlocks(sum uint64, latestBlockNumber uint64) {
	blockData := rpc.GetBlockByNumberMainnet(sum)
	if blockData == "" {
		return
	}

	var Zond models.Zond
	err := json.Unmarshal([]byte(blockData), &Zond)
	if err != nil {
		glog.Info("%v", err)
		return
	}

	if Zond.PreResult.ParentHash != "" {
		ZondNew := db.ConvertModelsUint64(Zond)
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
	db.ProcessTransactions(ZondNew)
	db.UpdateValidators(sum, previousHash)
}

func runPeriodicTask(task func(), interval time.Duration, taskName string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic in periodic task",
					zap.String("task", taskName),
					zap.Any("error", r))
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("Recovered from panic in periodic task tick",
							zap.String("task", taskName),
							zap.Any("error", r))
					}
				}()
				task()
			}()
		}
	}()
}

func singleBlockInsertion() {
	sum := db.GetLatestBlockNumberFromDB() + 1

	latestBlockNumber, err := rpc.GetLatestBlock()
	if err != nil {
		logger.Error("Failed to get latest block", zap.Error(err))
		return
	}

	isCollectionsExist := db.IsCollectionsExist()

	// Block processing goroutine
	runPeriodicTask(func() {
		if !isCollectionsExist {
			processInitialBlock()
			isCollectionsExist = true
		} else {
			if sum <= latestBlockNumber {
				processSubsequentBlocks(sum, latestBlockNumber)
				sum++
			} else {
				latestBlockNumber, err = rpc.GetLatestBlock()
				if err != nil {
					logger.Error("Failed to get latest block", zap.Error(err))
				}
			}
		}
	}, 60*time.Second, "block_processing")

	// Data update goroutine
	runPeriodicTask(func() {
		func() {
			db.PeriodicallyUpdateCoinGeckoData()
		}()
		func() {
			db.CountWallets()
		}()
		func() {
			db.GetDailyTransactionVolume()
		}()
	}, 5*time.Minute, "data_updates")

	select {}
}
