package db

import (
	"context"

	"backendAPI/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	canonicalBlockJoinAlias      = "__canonicalCompleteBlock"
	canonicalTokenBlockJoinAlias = "__canonicalCompleteTokenBlock"
)

func blocksCollectionName() string {
	if configs.BlocksCollection != nil {
		return configs.BlocksCollection.Name()
	}
	return "blocks"
}

// canonicalCompanionPipeline starts an aggregation that only admits rows whose
// blockNumber resolves to a durable, complete block. Rows with a missing block,
// a pending block, an unknown ingestion state, or a missing blockNumber all fail
// closed at the second $match.
func canonicalCompanionPipeline(filter interface{}) []bson.M {
	return append([]bson.M{{"$match": filter}}, canonicalCompleteBlockFenceStages("$blockNumber")...)
}

// canonicalTokenCompanionPipeline admits a token transfer only when its block
// height resolves to one durable block, token ingestion for that block is
// complete, and the transfer carries that block's exact hash identity. Legacy
// hashless rows fail closed and require an explicit reindex before publication.
func canonicalTokenCompanionPipeline(filter interface{}) []bson.M {
	return append(
		[]bson.M{{"$match": filter}},
		canonicalCompleteTokenBlockFenceStages("$blockNumber", "$blockHash")...,
	)
}

// canonicalCompleteBlockFenceStages joins only the minimum block identity needed
// to prove visibility. It requires exactly one block at the companion height and
// requires that block to be complete. A duplicate-height conflict therefore
// fails closed even if one of the conflicting rows carries a complete marker.
func canonicalCompleteBlockFenceStages(blockNumberExpression string) []bson.M {
	return []bson.M{
		{"$lookup": bson.M{
			"from": blocksCollectionName(),
			"let":  bson.M{"companionBlockNumber": blockNumberExpression},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$eq": bson.A{
					"$result.number", "$$companionBlockNumber",
				}}}},
				{"$limit": 2},
				{"$project": bson.M{"_id": 1, "ingestionState": 1}},
			},
			"as": canonicalBlockJoinAlias,
		}},
		{"$match": bson.M{"$expr": canonicalCompleteBlockJoinExpression()}},
	}
}

func canonicalCompleteBlockJoinExpression() bson.M {
	return bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{
			bson.M{"$size": "$" + canonicalBlockJoinAlias},
			1,
		}},
		bson.M{"$eq": bson.A{
			bson.M{"$arrayElemAt": bson.A{"$" + canonicalBlockJoinAlias + ".ingestionState", 0}},
			completedBlockIngestionState,
		}},
	}}
}

// canonicalCompleteTokenBlockFenceStages extends the core companion fence with
// the independently durable token-ingestion marker and exact block-hash
// identity. The lookup deliberately selects by height before comparing hashes:
// two rows at one height remain a conflict even when one hash matches.
func canonicalCompleteTokenBlockFenceStages(
	blockNumberExpression string,
	blockHashExpression string,
) []bson.M {
	return []bson.M{
		{"$lookup": bson.M{
			"from": blocksCollectionName(),
			"let":  bson.M{"companionBlockNumber": blockNumberExpression},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$eq": bson.A{
					"$result.number", "$$companionBlockNumber",
				}}}},
				{"$limit": 2},
				{"$project": bson.M{
					"_id":                 1,
					"ingestionState":      1,
					"tokenIngestionState": 1,
					"result.hash":         1,
				}},
			},
			"as": canonicalTokenBlockJoinAlias,
		}},
		{"$match": bson.M{"$expr": canonicalCompleteTokenBlockJoinExpression(blockHashExpression)}},
	}
}

func canonicalCompleteTokenBlockJoinExpression(blockHashExpression string) bson.M {
	return bson.M{"$and": bson.A{
		bson.M{"$eq": bson.A{
			bson.M{"$size": "$" + canonicalTokenBlockJoinAlias},
			1,
		}},
		bson.M{"$eq": bson.A{
			bson.M{"$arrayElemAt": bson.A{
				"$" + canonicalTokenBlockJoinAlias + ".ingestionState", 0,
			}},
			completedBlockIngestionState,
		}},
		bson.M{"$eq": bson.A{
			bson.M{"$arrayElemAt": bson.A{
				"$" + canonicalTokenBlockJoinAlias + ".tokenIngestionState", 0,
			}},
			completedBlockIngestionState,
		}},
		bson.M{"$eq": bson.A{
			bson.M{"$type": blockHashExpression},
			"string",
		}},
		bson.M{"$ne": bson.A{blockHashExpression, ""}},
		bson.M{"$eq": bson.A{
			blockHashExpression,
			bson.M{"$arrayElemAt": bson.A{
				"$" + canonicalTokenBlockJoinAlias + ".result.hash", 0,
			}},
		}},
	}}
}

// canonicalContractPipeline admits a contract only when its creation block is
// complete. Genesis contracts are explicit protocol state and have no creation
// transaction, so their persisted genesisContract marker is the sole exception.
func canonicalContractPipeline(filter interface{}) []bson.M {
	pipeline := []bson.M{{"$match": filter}}
	pipeline = append(pipeline, canonicalCompleteBlockFenceStages("$creationBlockNumber")...)
	pipeline[len(pipeline)-1] = bson.M{"$match": bson.M{"$or": bson.A{
		bson.M{"genesisContract": true},
		bson.M{"$expr": canonicalCompleteBlockJoinExpression()},
	}}}
	return pipeline
}

func aggregateCanonicalCount(
	ctx context.Context,
	collection *mongo.Collection,
	pipeline []bson.M,
) (int64, error) {
	countPipeline := make([]bson.M, 0, len(pipeline)+1)
	countPipeline = append(countPipeline, pipeline...)
	countPipeline = append(countPipeline, bson.M{"$count": "total"})
	cursor, err := collection.Aggregate(ctx, countPipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var result struct {
		Total int64 `bson:"total"`
	}
	if !cursor.Next(ctx) {
		if err := cursor.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if err := cursor.Decode(&result); err != nil {
		return 0, err
	}
	return result.Total, nil
}
