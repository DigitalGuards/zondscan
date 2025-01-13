package db

// import (
// 	"QRLtoMongoDB/models"
// 	"fmt"
// )

// func ProcessAverageBlockSize(blockData []interface{}) {
// 	for _, data := range blockData {
// 		field, _ := data.(models.ZondUint64Version)
// 		if field.Result ==  {
// 			fmt.Println("sjdfjsf")
// 		}
// 	}

// 	// groupStage := bson.D{
// 	// 	{"$group", bson.D{
// 	// 		{"_id", bson.D{
// 	// 			{"$dateToString", bson.D{
// 	// 				{"format", "%Y-%m-%d"},
// 	// 				{"date", bson.D{{"$toDate", bson.D{{"$getField", "result.timestamp.$numberLong"}}}}},
// 	// 			}},
// 	// 		}},
// 	// 		{"average_size", bson.D{{"$avg", bson.D{{"$getField", "result.size"}}}}},
// 	// 	}},
// 	// }

// 	// outStage := bson.D{
// 	// 	{"$out", "average_block_size"},
// 	// }

// 	// opts := options.Aggregate().SetAllowDiskUse(true)
// 	// cur, err := configs.BlocksCollections.Aggregate(ctx, mongo.Pipeline{matchStage, groupStage, outStage}, opts)
// 	// if err != nil {
// 	// 	log.Fatal(err)
// 	// }
// 	// defer cur.Close(ctx)

// 	// fmt.Println("Aggregation completed and results stored in 'average_block_size' collection.")
// }
