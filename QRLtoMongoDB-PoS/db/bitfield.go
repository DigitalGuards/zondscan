package db

import (
	"QRLtoMongoDB-PoS/bitfield"
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"context"
	"math/big"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func processXMSSBitfield(from string, signature string) {
	if from == "0x1" {
		otsKeyIndex := new(big.Int)
		otsKeyIndex.SetString(signature, 16)

		var wallet models.Wallet

		if wallet.Paged == nil {
			wallet.Paged = bitfield.NewBig()
		}

		wallet.Paged.Set(otsKeyIndex)

		pageNumber := new(big.Int).Div(otsKeyIndex, big.NewInt(1024))
		for _, binData := range wallet.Paged {
			insertBitfieldByteArray(from+"_"+pageNumber.String(), binData)
		}
	}
}

func insertBitfieldByteArray(address_pagenumber string, ots_bitfield bitfield.Bitfield) (*mongo.InsertOneResult, error) {
	doc := bson.D{{"address_pagenumber", address_pagenumber}, {"ots_bitfield", ots_bitfield}}
	result, err := configs.BitfieldCollections.InsertOne(context.TODO(), doc)
	if err != nil {
		configs.Logger.Warn("Failed to insert in the bitfield collection: ", zap.Error(err))
	}

	return result, err
}

func GetAddressPageNumber(address string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var addresses []models.Address
	defer cancel()

	options := options.Find().SetSort(bson.D{{"address_pagenumber", -1}}).SetLimit(1)

	regexQuery := "\\w{39}_[0-9][0-9]"
	results, err := configs.BitfieldCollections.Find(ctx, bson.M{"address_pagenumber": bson.M{"$regex": regexQuery}}, options)

	if err != nil {
		configs.Logger.Warn("Failed to do Bitfield find: ", zap.Error(err))
	}

	//reading from the db in an optimal way
	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleAddress models.Address
		if err = results.Decode(&singleAddress); err != nil {
			configs.Logger.Warn("Failed to decode: ", zap.Error(err))
		}

		addresses = append(addresses, singleAddress)
	}

	var result string
	if len(addresses) > 1 {
		pageNumber, err := strconv.Atoi(addresses[0].ID[40:])
		if err != nil {
			configs.Logger.Warn("Failed to do ParseInt", zap.Error(err))
		}
		pageNumberStr := strconv.Itoa((pageNumber + 1))
		result = addresses[0].ID[40:] + pageNumberStr
	} else {
		result = address + "_00"
	}

	return result, nil
}
