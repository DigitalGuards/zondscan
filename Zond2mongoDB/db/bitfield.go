package db

import (
	"Zond2mongoDB/bitfield"
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"context"
	"math/big"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
	doc := primitive.D{
		{Key: "address_pagenumber", Value: address_pagenumber},
		{Key: "ots_bitfield", Value: ots_bitfield},
	}
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

	sortOpt := primitive.D{{Key: "address_pagenumber", Value: -1}}
	options := options.Find().SetSort(sortOpt).SetLimit(1)

	regexQuery := "\\w{39}_[0-9][0-9]"
	filter := primitive.D{
		{Key: "address_pagenumber", Value: primitive.D{
			{Key: "$regex", Value: regexQuery},
		}},
	}

	results, err := configs.BitfieldCollections.Find(ctx, filter, options)
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
