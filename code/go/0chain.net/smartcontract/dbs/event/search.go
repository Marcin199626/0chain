package event

import (
	"database/sql"
	"strconv"
)

// GetGenericSearchType returns kind of search query
func (edb *EventDb) GetGenericSearchType(query string) (string, error) {
	var queryType string
	var round int

	round, err := strconv.Atoi(query)
	if err != nil {
		round = -1
	}

	res := edb.Store.Get().Raw(
		"SELECT "+
			"CASE "+
			"WHEN EXISTS (SELECT Id FROM TRANSACTIONS WHERE HASH = @query) "+
			"THEN 'TransactionHash' "+
			"WHEN EXISTS (SELECT Id FROM BLOCKS WHERE HASH = @query) "+
			"THEN 'BlockHash' "+
			"WHEN EXISTS (SELECT Id FROM USERS WHERE USER_ID = @query) "+
			"THEN 'UserId' "+
			"WHEN EXISTS (SELECT Id FROM BLOCKS WHERE ROUND = @round) "+
			"THEN 'BlockRound' "+
			"WHEN EXISTS (SELECT Id FROM WRITE_MARKERS WHERE CONTENT_HASH = @query) "+
			"THEN 'ContentHash' "+
			"WHEN EXISTS (SELECT Id FROM WRITE_MARKERS WHERE NAME = @query) "+
			"THEN 'FileName' "+
			"ELSE 'Not Found' "+
			"END AS queryType",
		sql.Named("query", query),
		sql.Named("round", round),
	).Scan(&queryType)

	return queryType, res.Error
}
